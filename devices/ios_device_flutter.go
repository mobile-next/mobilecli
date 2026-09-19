package devices

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios/installationproxy"
	iosutil "github.com/mobile-next/mobilecli/devices/ios"
	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

// Flutter support for a real iOS device. The render-tree walk is the same Dart
// as the simulator and Android (same _ReusableRenderView root, same
// invoke/localToGlobal), so this only handles the device-specific parts:
//   - Getting the Dart VM service URI: the injected in-process agent
//     (agents/ios-real/agent.m) reads it from the running Flutter engine and
//     returns it over the existing agentCall channel — no logs, no mDNS.
//   - Reaching it: the VM service listens on the device's loopback, so we
//     forward a local port to it through the same go-ios tunnel the agent uses.
//
// Detection happens in two steps. Injecting the agent means attaching LLDB,
// which pauses the app and takes ~20s on any debuggable app, so it is only
// attempted when the foreground app's Info.plist advertises the Dart VM service
// over bonjour — Flutter adds that to debug and profile builds, the only builds
// that have a VM service to read. After that, detection is by attempt: a
// non-Flutter app answers the RPC with "not a flutter app", and we fall back to
// the accessibility dump. Like the simulator, iOS reports layout in logical
// points, so dpr = 1.0.

// bonjour service types the Flutter tool adds to debug/profile builds so that
// `flutter attach` can find the VM service; the second is what older Flutter versions called it.
// how long to wait for a bonjour answer before falling back to LLDB.
const dartVMServiceMDNSTimeout = 3 * time.Second

var dartVMServiceTypes = []string{"_dartVmService._tcp", "_dartobservatory._tcp"}

// flutterCandidate remembers the verdict for one process: its Info.plist cannot
// change while it runs, and a reinstall gets a new pid.
type flutterCandidate struct {
	pid         int
	isCandidate bool
}

// advertisesDartVMService reports whether an installed app's Info.plist declares
// the Dart VM bonjour service.
func advertisesDartVMService(app installationproxy.AppInfo) bool {
	services, ok := app["NSBonjourServices"].([]any)
	if !ok {
		return false
	}
	for _, service := range services {
		name, _ := service.(string)
		for _, dartService := range dartVMServiceTypes {
			if name == dartService {
				return true
			}
		}
	}
	return false
}

// foregroundFlutterCandidate decides whether looking for a Dart VM is worth it,
// and names the foreground app when it knows it. when the answer cannot be
// determined it says yes with no bundle id, which is the old probe-over-LLDB
// behaviour.
func (d *IOSDevice) foregroundFlutterCandidate() (bundleID string, mayBeFlutter bool) {
	activeApp, err := d.deviceKitClient.GetActiveAppInfo()
	if err != nil || activeApp.ProcessID == 0 {
		utils.Verbose("flutter: cannot tell the foreground app, probing anyway: %v", err)
		return "", true
	}

	d.mu.Lock()
	cached := d.flutterCandidate
	d.mu.Unlock()
	if cached.pid == activeApp.ProcessID {
		return activeApp.BundleID, cached.isCandidate
	}

	isCandidate, err := d.installedAppAdvertisesDartVMService(activeApp.BundleID)
	if err != nil {
		utils.Verbose("flutter: cannot read %s's Info.plist, probing anyway: %v", activeApp.BundleID, err)
		return activeApp.BundleID, true
	}

	d.mu.Lock()
	d.flutterCandidate = flutterCandidate{pid: activeApp.ProcessID, isCandidate: isCandidate}
	d.mu.Unlock()
	return activeApp.BundleID, isCandidate
}

// installedAppAdvertisesDartVMService looks the bundle up among user apps; system
// apps are not listed there and are never Flutter.
func (d *IOSDevice) installedAppAdvertisesDartVMService(bundleID string) (bool, error) {
	device, err := d.getEnhancedDevice()
	if err != nil {
		return false, fmt.Errorf("get enhanced device: %w", err)
	}
	svc, err := installationproxy.New(device)
	if err != nil {
		return false, fmt.Errorf("installationproxy: %w", err)
	}
	defer svc.Close()

	apps, err := svc.BrowseUserApps()
	if err != nil {
		return false, fmt.Errorf("browse user apps: %w", err)
	}
	for _, app := range apps {
		if app.CFBundleIdentifier() == bundleID {
			return advertisesDartVMService(app), nil
		}
	}
	return false, nil
}

// tryDumpFlutterSource returns the Flutter render tree for the foreground app,
// or ok=false to signal the caller should use the accessibility dump.
func (d *IOSDevice) tryDumpFlutterSource() ([]types.ScreenElement, bool) {
	bundleID, mayBeFlutter := d.foregroundFlutterCandidate()
	if !mayBeFlutter {
		utils.Verbose("flutter: foreground app does not advertise a Dart VM service, skipping the probe")
		return nil, false
	}

	// the engine advertises its VM service over bonjour, and that reaches the
	// Mac over the usb link too: ~1s, no attach, the app keeps running.
	if uri := d.dartVMServiceURIViaMDNS(bundleID); uri != "" {
		elements, err := d.dumpFlutterSourceDevice(uri)
		if err != nil {
			// the agent could only hand us this same URI, so injecting it over
			// LLDB would cost ~20s and fail the same way
			utils.Verbose("flutter: render-tree dump failed, falling back: %v", err)
			return nil, false
		}
		return elements, true
	}

	// slow path: inject the agent over LLDB (~20s, pauses the app) and ask it
	raw, err := d.agentCall("device.flutter.vmServiceUri", nil)
	if err != nil {
		// Expected for non-Flutter apps ("not a flutter app") and when the
		// agent/tunnel isn't available; both mean "use the accessibility dump".
		utils.Verbose("flutter: vmServiceUri unavailable: %v", err)
		return nil, false
	}
	var r struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(raw, &r); err != nil || r.URI == "" {
		return nil, false
	}

	elements, err := d.dumpFlutterSourceDevice(r.URI)
	if err != nil {
		utils.Verbose("flutter: render-tree dump failed, falling back: %v", err)
		return nil, false
	}
	return elements, true
}

// dartVMServiceURIViaMDNS finds the VM service the way `flutter attach` and the
// simulator path do, or returns "" when the app is not advertising one (it was
// denied Local Network access, or the engine has publication turned off).
// ponytail: the instance name is just the bundle id, so with several phones
// running the same app the first answer may be another phone's. its port and
// auth code then fail through this device's tunnel and the dump falls back to
// the accessibility tree; resolve every answer and try each if farms hit this.
func (d *IOSDevice) dartVMServiceURIViaMDNS(bundleID string) string {
	if bundleID == "" {
		return ""
	}
	uri := resolveDartVMServiceMDNS(bundleID, dartVMServiceMDNSTimeout)
	if uri == "" {
		utils.Verbose("flutter: %s is not advertising a Dart VM service over mDNS", bundleID)
	}
	return uri
}

// dumpFlutterSourceDevice forwards a local port to the device's VM service port
// and walks the render tree over it.
func (d *IOSDevice) dumpFlutterSourceDevice(uri string) ([]types.ScreenElement, error) {
	m := vmServiceURIPattern.FindStringSubmatch(strings.TrimSpace(uri))
	if m == nil {
		return nil, fmt.Errorf("unexpected Dart VM service URI: %q", uri)
	}
	devicePort, err := strconv.Atoi(m[1])
	if err != nil {
		return nil, fmt.Errorf("bad VM service port %q: %w", m[1], err)
	}
	token := m[2]

	localPort, err := freeLocalPort()
	if err != nil {
		return nil, err
	}
	pf := iosutil.NewPortForwarder(d.Udid)
	if err := pf.Forward(localPort, devicePort); err != nil {
		return nil, fmt.Errorf("forward VM service port %d: %w", devicePort, err)
	}
	defer pf.Stop() //nolint:errcheck

	// token is empty when the app was launched with --disable-service-auth-codes.
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", localPort)
	if token != "" {
		wsURL = fmt.Sprintf("ws://127.0.0.1:%d/%s/ws", localPort, token)
	}

	start := time.Now()
	elements, err := dumpFlutterTreeOverWS(wsURL, 1.0)
	if err != nil {
		return nil, err
	}
	utils.Verbose("flutter: render-tree dump produced %d elements in %s", len(elements), time.Since(start))
	return elements, nil
}
