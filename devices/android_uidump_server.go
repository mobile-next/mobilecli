package devices

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/agents"
	"github.com/mobile-next/mobilecli/utils"
)

// deviceKitServerTarget is the localabstract socket UiDumpServer
// (agents/android/java/UiDumpServer.java) binds once launched. Reusing one
// persistent process avoids paying process-fork and UiAutomation-connect cost
// on every dump ui call.
const deviceKitServerTarget = "localabstract:mobilecli-uidump"

// deviceKitServerWaitUntilIdleMs is how long a dump waits for the UI to settle.
const deviceKitServerWaitUntilIdleMs = 2000

// ensureDeviceKitServerReady makes sure the persistent UiDumpServer is running
// and forwarded, reusing an existing forward when possible. It's launched
// detached (nohup + &) on the device shell so it outlives this CLI invocation
// and keeps serving fast dumps for subsequent mobilecli calls.
func (d *AndroidDevice) ensureDeviceKitServerReady() (int, error) {
	if port := d.findForward(deviceKitServerTarget); port != 0 && isAgentReady(port) {
		return port, nil
	}

	if err := d.pushTempFile(agents.AndroidMobilecliDEX, androidDexPath); err != nil {
		return 0, fmt.Errorf("push .dex: %w", err)
	}

	// Only one UiAutomation may be registered system-wide: a stale server (or the
	// legacy devicekit instrumentation from older mobilecli versions) would block
	// the new one from connecting, so clear both first. This runs as its own adb
	// shell so pkill -f can't match the launch command line below (or itself).
	_, _ = d.runAdbCommand("shell", "pkill -f '[U]iDumpServer'; pkill -f 'com.mobilenext.[d]evicekit'; true")

	// nohup+& detaches the server from this adb shell session so it survives
	// after this command returns.
	launchCmd := fmt.Sprintf("CLASSPATH=%s nohup app_process / com.mobilenext.mobilecli.UiDumpServer >/dev/null 2>&1 &", androidDexPath)
	if out, err := d.runAdbCommand("shell", launchCmd); err != nil {
		return 0, fmt.Errorf("launch ui dump server: %s: %w", strings.TrimSpace(string(out)), err)
	}

	port, err := d.addForward(deviceKitServerTarget)
	if err != nil {
		return 0, err
	}

	deadline := time.Now().Add(5 * time.Second)
	for !isAgentReady(port) {
		if time.Now().After(deadline) {
			d.removeForward(port)
			return 0, fmt.Errorf("devicekit server did not start within 5s on port %d", port)
		}
		time.Sleep(100 * time.Millisecond)
	}

	return port, nil
}

// addForward creates a host TCP forward to target (e.g.
// "localabstract:devicekit") on a freshly assigned local port and returns it.
func (d *AndroidDevice) addForward(target string) (int, error) {
	out, err := d.runAdbCommand("forward", "tcp:0", target)
	if err != nil {
		return 0, fmt.Errorf("adb forward: %s: %w", strings.TrimSpace(string(out)), err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("unexpected adb forward output %q: %w", strings.TrimSpace(string(out)), err)
	}
	return port, nil
}

// removeForward tears down a host TCP forward created by this device, best
// effort — used to avoid leaking a forward when the server it points at never
// became ready.
func (d *AndroidDevice) removeForward(port int) {
	if _, err := d.runAdbCommand("forward", "--remove", fmt.Sprintf("tcp:%d", port)); err != nil {
		utils.Verbose("failed to remove stale forward on port %d: %v", port, err)
	}
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
	if len(hierarchy.Hierarchy) == 0 {
		return nil, fmt.Errorf("no hierarchy found in devicekit server dump")
	}

	utils.Verbose("getDeviceKitServerNodes took %s", time.Since(startTime))
	return hierarchy.Hierarchy, nil
}
