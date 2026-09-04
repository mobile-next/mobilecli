package devices

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/agents"
	"github.com/mobile-next/mobilecli/utils"
)

// deviceServerClass is the persistent on-device server (agents/android/java/DeviceServer.java).
const deviceServerClass = "com.mobilenext.mobilecli.DeviceServer"

// deviceServerTarget is the localabstract socket DeviceServer binds once
// launched. Reusing one persistent process avoids paying process-fork and
// UiAutomation-connect cost on every call.
const deviceServerTarget = "localabstract:mobilecli-server"

// dumpUiWaitUntilIdleMs is how long a dump waits for the UI to settle.
const dumpUiWaitUntilIdleMs = 2000

// ensureDeviceServerReady makes sure the persistent DeviceServer is running,
// forwarded, and built from this binary's dex, reusing an existing forward when
// possible. It's launched detached (nohup + &) on the device shell so it
// outlives this CLI invocation and keeps serving subsequent mobilecli calls.
func (d *AndroidDevice) ensureDeviceServerReady() (int, error) {
	if port := d.findForward(deviceServerTarget); port != 0 && isAgentReady(port) {
		if d.deviceServerMatchesEmbeddedDex(port) {
			return port, nil
		}
		utils.Verbose("device server is from another mobilecli build, restarting it")
		d.removeForward(port)
	}

	if err := d.pushTempFile(agents.AndroidMobilecliDEX, androidDexPath); err != nil {
		return 0, fmt.Errorf("push .dex: %w", err)
	}

	// Only one UiAutomation may be registered system-wide: a stale server (or the
	// legacy devicekit instrumentation from older mobilecli versions) would block
	// the new one from connecting, so clear both first. This runs as its own adb
	// shell so pkill -f can't match the launch command line below (or itself).
	_, _ = d.runAdbCommand("shell", "pkill -f '[D]eviceServer'; pkill -f '[U]iDumpServer'; pkill -f 'com.mobilenext.[d]evicekit'; true")

	// nohup+& detaches the server from this adb shell session so it survives
	// after this command returns.
	launchCmd := fmt.Sprintf("CLASSPATH=%s nohup app_process / %s >/dev/null 2>&1 &", androidDexPath, deviceServerClass)
	if out, err := d.runAdbCommand("shell", launchCmd); err != nil {
		return 0, fmt.Errorf("launch device server: %s: %w", strings.TrimSpace(string(out)), err)
	}

	port, err := d.addForward(deviceServerTarget)
	if err != nil {
		return 0, err
	}

	deadline := time.Now().Add(5 * time.Second)
	for !isAgentReady(port) {
		if time.Now().After(deadline) {
			d.removeForward(port)
			return 0, fmt.Errorf("device server did not start within 5s on port %d", port)
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

// deviceServerMatchesEmbeddedDex reports whether the running server was started
// from the dex embedded in this binary. A server left over from an older
// mobilecli build would otherwise keep serving old code indefinitely.
func (d *AndroidDevice) deviceServerMatchesEmbeddedDex(port int) bool {
	raw, err := agentRequest(port, "device.version", nil)
	if err != nil {
		return false
	}
	var version struct {
		DexSHA256 string `json:"dexSha256"`
	}
	if err := json.Unmarshal(raw, &version); err != nil {
		return false
	}
	return version.DexSHA256 == embeddedDexSHA256()
}

func embeddedDexSHA256() string {
	sum := sha256.Sum256(agents.AndroidMobilecliDEX)
	return hex.EncodeToString(sum[:])
}

// serverRequest sends a JSON-RPC call to the persistent DeviceServer, starting
// it if needed.
func (d *AndroidDevice) serverRequest(method string, params map[string]any) (json.RawMessage, error) {
	port, err := d.ensureDeviceServerReady()
	if err != nil {
		return nil, err
	}
	return agentRequest(port, method, params)
}

// dumpUiNodes fetches the UI hierarchy from the DeviceServer. Callers fall back
// to uiautomator when it's unavailable.
func (d *AndroidDevice) dumpUiNodes() ([]uiNode, error) {
	startTime := time.Now()

	raw, err := d.serverRequest("device.dump.ui", map[string]any{"waitUntilIdle": dumpUiWaitUntilIdleMs})
	if err != nil {
		return nil, fmt.Errorf("device server dump.ui: %w", err)
	}

	var hierarchy uiHierarchy
	if err := json.Unmarshal(raw, &hierarchy); err != nil {
		return nil, fmt.Errorf("parse device server response: %w", err)
	}
	if len(hierarchy.Hierarchy) == 0 {
		return nil, fmt.Errorf("no hierarchy found in device server dump")
	}

	utils.Verbose("dumpUiNodes took %s", time.Since(startTime))
	return hierarchy.Hierarchy, nil
}
