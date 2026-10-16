package devices

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

// Flutter support for the iOS simulator. The render-tree walk is identical Dart
// to Android (same `_ReusableRenderView` root, same `invoke`/localToGlobal), so
// the whole flutterVM client and dumpRenderTree are reused verbatim. Only two
// things are iOS-specific: detecting Flutter (the app bundle embeds
// Flutter.framework) and obtaining the Dart VM service URI. On the simulator the
// app runs natively on the Mac, so its VM service listens on the Mac's own
// 127.0.0.1 — we connect directly, no port forwarding and no code injection.
//
// iOS reports layout in logical points (matching the existing DeviceKit dump),
// so unlike Android we do not scale by the device pixel ratio (dpr = 1.0).

// vmServiceLineURL pulls the service URL out of the engine's log line
// "The Dart VM service is listening on http://127.0.0.1:PORT/TOKEN/".
var vmServiceLineURL = regexp.MustCompile(`http://127\.0\.0\.1:\d+/[A-Za-z0-9_=-]*/`)

// tryDumpFlutterSource returns the Flutter render tree for the foreground app,
// or ok=false to signal the caller should use the accessibility dump.
func (s *SimulatorDevice) tryDumpFlutterSource() ([]types.ScreenElement, bool) {
	bundleID, mayBeFlutter := s.foregroundFlutterCandidate()
	if !mayBeFlutter {
		return nil, false
	}
	uri := s.flutterVMServiceURI(bundleID)
	if uri == "" {
		utils.Verbose("flutter: no Dart VM service URI found for %s (release build, or the launch log rotated out)", bundleID)
		return nil, false
	}
	start := time.Now()
	elements, err := dumpFlutterSourceFromURI(uri, 1.0)
	if err != nil {
		utils.Verbose("flutter: render-tree dump failed, falling back: %v", err)
		return nil, false
	}
	utils.Verbose("flutter: render-tree dump produced %d elements in %s", len(elements), time.Since(start))
	return elements, true
}

// simulatorFlutterCandidates caches the verdict of foregroundFlutterCandidate,
// keyed by simulator udid. SimulatorDevice methods run against a copy of the
// device, so the cache cannot live in the struct the way it does for a real
// device.
var (
	simulatorFlutterMu         sync.Mutex
	simulatorFlutterCandidates = map[string]flutterCandidate{}
)

// cachedFlutterCandidate returns the verdict remembered for the process running
// on a simulator, and whether there is one. A different pid means the app was
// relaunched or reinstalled, which is a miss: the bundle may be another build.
func cachedFlutterCandidate(udid string, pid int) (isCandidate bool, ok bool) {
	if pid == 0 {
		return false, false
	}

	simulatorFlutterMu.Lock()
	defer simulatorFlutterMu.Unlock()

	cached, exists := simulatorFlutterCandidates[udid]
	if !exists || cached.pid != pid {
		return false, false
	}
	return cached.isCandidate, true
}

// rememberFlutterCandidate stores the verdict for a process. A pid of 0 means
// the agent did not report one, and a verdict that cannot be invalidated is
// worse than no verdict at all, so it is dropped.
func rememberFlutterCandidate(udid string, pid int, isCandidate bool) {
	if pid == 0 {
		return
	}

	simulatorFlutterMu.Lock()
	defer simulatorFlutterMu.Unlock()
	simulatorFlutterCandidates[udid] = flutterCandidate{pid: pid, isCandidate: isCandidate}
}

// foregroundFlutterCandidate decides whether looking for a Dart VM is worth it,
// and names the foreground app. Reading the app bundle costs a simctl call, more
// than the whole accessibility dump of a light screen, and every dump would pay
// it; the answer cannot change while the process runs, so it is remembered per
// process id, as on a real device. A relaunched or reinstalled app gets a new
// pid and is looked at again.
func (s *SimulatorDevice) foregroundFlutterCandidate() (bundleID string, mayBeFlutter bool) {
	activeApp, err := s.deviceKitClient.GetActiveAppIdentity()
	if err != nil || activeApp.BundleID == "" {
		utils.Verbose("flutter: cannot tell the foreground app, skipping the flutter dump: %v", err)
		return "", false
	}

	if isCandidate, ok := cachedFlutterCandidate(s.UDID, activeApp.ProcessID); ok {
		return activeApp.BundleID, isCandidate
	}

	isCandidate := s.isFlutterAppBundle(activeApp.BundleID)
	rememberFlutterCandidate(s.UDID, activeApp.ProcessID, isCandidate)

	return activeApp.BundleID, isCandidate
}

// isFlutterAppBundle reports whether the installed app embeds Flutter.framework.
func (s *SimulatorDevice) isFlutterAppBundle(bundleID string) bool {
	out, err := runSimctl("get_app_container", s.UDID, bundleID, "app")
	if err != nil {
		return false
	}
	appPath := strings.TrimSpace(string(out))
	if appPath == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(appPath, "Frameworks", "Flutter.framework"))
	return err == nil && info.IsDir()
}

// flutterVMServiceURI recovers the running app's Dart VM service URI (with its
// auth token). Primary source is mDNS: the Flutter engine advertises
// `_dartVmService._tcp` (instance name = bundle id) with the port and auth code
// in its TXT record, for as long as the app runs — this is what `flutter attach`
// uses, ~5ms, and survives log rotation. The simulator log is a fallback for the
// rare case where VM-service publication is disabled.
func (s *SimulatorDevice) flutterVMServiceURI(bundleID string) string {
	if uri := resolveDartVMServiceMDNS(bundleID, 3*time.Second); uri != "" {
		return uri
	}
	utils.Verbose("flutter: mDNS lookup failed for %s, trying simulator log", bundleID)
	return s.flutterVMServiceURIFromLog()
}

var (
	mdnsPortLine = regexp.MustCompile(`can be reached at \S+?:(\d+)`)
	mdnsAuthCode = regexp.MustCompile(`authCode=([A-Za-z0-9_=+/\-]+)`)
)

// resolveDartVMServiceMDNS resolves the app's Dart VM service via Bonjour and
// returns http://127.0.0.1:<port>/<authCode>/, or "" if not found in time.
func resolveDartVMServiceMDNS(bundleID string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// -L resolves a specific instance (the bundle id) to host:port + TXT record.
	// dns-sd streams until killed, so we read until we have both fields. Use the
	// absolute system path rather than relying on PATH.
	cmd := exec.CommandContext(ctx, "/usr/bin/dns-sd", "-L", bundleID, "_dartVmService._tcp", "local.")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ""
	}
	if err := cmd.Start(); err != nil {
		return ""
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	var port, auth string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if m := mdnsPortLine.FindStringSubmatch(line); m != nil {
			port = m[1]
		}
		if m := mdnsAuthCode.FindStringSubmatch(line); m != nil {
			auth = m[1]
		}
		if port != "" && auth != "" {
			return fmt.Sprintf("http://127.0.0.1:%s/%s/", port, auth)
		}
	}
	return ""
}

// flutterVMServiceURIFromLog scrapes the URI (with token) from the simulator log,
// where the engine prints it once at launch. Fragile — the line rotates out of
// the log buffer over time — so it is only a fallback for mDNS.
func (s *SimulatorDevice) flutterVMServiceURIFromLog() string {
	out, err := runSimctl("spawn", s.UDID, "log", "show", "--last", "30m",
		"--style", "compact", "--predicate", `eventMessage CONTAINS "Dart VM service is listening"`)
	if err != nil {
		utils.Verbose("flutter: reading simulator log failed: %v", err)
		return ""
	}
	matches := vmServiceLineURL.FindAllString(string(out), -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1] // newest listener wins
}

// dumpFlutterSourceFromURI connects to a Dart VM service already reachable at
// uri's host:port (the iOS-simulator case — the app runs on the Mac) and walks
// the render tree. dpr is 1.0 for iOS (points) and the device pixel ratio for
// Android (physical pixels).
func dumpFlutterSourceFromURI(uri string, dpr float64) ([]types.ScreenElement, error) {
	m := vmServiceURIPattern.FindStringSubmatch(strings.TrimSpace(uri))
	if m == nil {
		return nil, fmt.Errorf("unexpected Dart VM service URI: %q", uri)
	}
	// m[2] is the auth token; it is empty when the app was launched with
	// --disable-service-auth-codes (the no-log variant), which changes the path.
	wsURL := fmt.Sprintf("ws://127.0.0.1:%s/ws", m[1])
	if m[2] != "" {
		wsURL = fmt.Sprintf("ws://127.0.0.1:%s/%s/ws", m[1], m[2])
	}
	return dumpFlutterTreeOverWS(wsURL, dpr)
}

// dumpFlutterTreeOverWS is the platform-neutral core: dial the Dart VM service
// WebSocket, bind the isolate, and walk the render tree. Android reaches it
// through an adb-forwarded local port; iOS connects to the Mac directly.
func dumpFlutterTreeOverWS(wsURL string, dpr float64) ([]types.ScreenElement, error) {
	vm, err := dialFlutterVM(wsURL)
	if err != nil {
		return nil, err
	}
	defer vm.close()
	if err := vm.resolveIsolate(); err != nil {
		return nil, err
	}
	return vm.dumpRenderTree(dpr)
}
