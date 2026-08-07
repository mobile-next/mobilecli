package devices

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

// deviceKitServerTarget is the localabstract socket DeviceKitServer binds once
// launched. Reusing one persistent instrumentation avoids paying process-fork
// and UiAutomation-connect cost on every dump ui call.
const deviceKitServerTarget = "localabstract:devicekit"

// deviceKitServerWaitUntilIdleMs matches the idle wait used by the legacy
// one-shot ViewTreeDump instrumentation, so dumps settle the same way.
const deviceKitServerWaitUntilIdleMs = 2000

// ensureDeviceKitServerReady makes sure the persistent DeviceKitServer
// instrumentation is running and forwarded, reusing an existing forward when
// possible. It's launched detached (nohup + &) on the device shell so it
// outlives this CLI invocation and keeps serving fast dumps for subsequent
// mobilecli calls.
func (d *AndroidDevice) ensureDeviceKitServerReady() (int, error) {
	if port := d.findForward(deviceKitServerTarget); port != 0 && isAgentReady(port) {
		return port, nil
	}

	if err := d.EnsureDeviceKitInstalled(); err != nil {
		return 0, fmt.Errorf("ensure devicekit installed: %w", err)
	}

	// -w keeps `am` blocked (and its UiAutomationConnection alive) for as long as
	// DeviceKitServer runs; nohup+& detaches it from this adb shell session so it
	// survives after this command returns.
	launchCmd := "nohup am instrument -w com.mobilenext.devicekit/.DeviceKitServer >/dev/null 2>&1 &"
	if out, err := d.runAdbCommand("shell", launchCmd); err != nil {
		return 0, fmt.Errorf("launch devicekit server: %s: %w", strings.TrimSpace(string(out)), err)
	}

	out, err := d.runAdbCommand("forward", "tcp:0", deviceKitServerTarget)
	if err != nil {
		return 0, fmt.Errorf("adb forward: %s: %w", strings.TrimSpace(string(out)), err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("unexpected adb forward output %q: %w", strings.TrimSpace(string(out)), err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for !isAgentReady(port) {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("devicekit server did not start within 5s on port %d", port)
		}
		time.Sleep(100 * time.Millisecond)
	}

	return port, nil
}

// getDeviceKitServerNodes fetches the UI hierarchy from the persistent
// DeviceKitServer. This is the fast path for DumpSourceRaw/DumpSource; callers
// fall back to the one-shot instrumentation when it's unavailable.
func (d *AndroidDevice) getDeviceKitServerNodes() ([]deviceKitNode, error) {
	startTime := time.Now()

	port, err := d.ensureDeviceKitServerReady()
	if err != nil {
		return nil, err
	}

	raw, err := agentRequest(port, "device.dump.ui", map[string]any{"waitUntilIdle": deviceKitServerWaitUntilIdleMs})
	if err != nil {
		return nil, fmt.Errorf("devicekit server dump.ui: %w", err)
	}

	var hierarchy deviceKitHierarchy
	if err := json.Unmarshal(raw, &hierarchy); err != nil {
		return nil, fmt.Errorf("parse devicekit server response: %w", err)
	}

	utils.Verbose("getDeviceKitServerNodes took %s", time.Since(startTime))
	return hierarchy.Hierarchy, nil
}
