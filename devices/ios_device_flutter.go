package devices

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

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
// Detection is by attempt: a non-Flutter foreground app answers the RPC with
// "not a flutter app", and we fall back to the accessibility dump. Like the
// simulator, iOS reports layout in logical points, so dpr = 1.0.

// tryDumpFlutterSource returns the Flutter render tree for the foreground app,
// or ok=false to signal the caller should use the accessibility dump.
func (d *IOSDevice) tryDumpFlutterSource() ([]types.ScreenElement, bool) {
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
