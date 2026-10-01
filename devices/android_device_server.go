package devices

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mobile-next/mobilecli/agents"
	"github.com/mobile-next/mobilecli/utils"
)

// dumpUiParams is what DeviceServer's device.dump.ui reads.
type dumpUiParams struct {
	WaitUntilIdle int  `json:"waitUntilIdle"`
	Full          bool `json:"full"`
}

// deviceServerClass is the persistent on-device server (agents/android/java/DeviceServer.java).
const deviceServerClass = "com.mobilenext.mobilecli.DeviceServer"

// deviceServerKillCommand stops the server and whatever older mobilecli
// versions left holding the UiAutomation (the UiDumpServer and the devicekit
// instrumentation). Killing the server also drops any mock-location test
// providers it held, so the mock_location appop granted for them goes back too.
const deviceServerKillCommand = "pkill -f '[D]eviceServer'; pkill -f '[U]iDumpServer'; pkill -f 'com.mobilenext.[d]evicekit'; " +
	"appops set " + shellPackage + " android:mock_location default; true"

// deviceServerTarget is the localabstract socket DeviceServer binds once
// launched. Reusing one persistent process avoids paying process-fork and
// UiAutomation-connect cost on every call.
const deviceServerTarget = "localabstract:mobilecli-server"

// dumpUiWaitUntilIdleMs is how long a dump waits for the UI to settle.
const dumpUiWaitUntilIdleMs = 2000

// deviceServerIdleTimeoutEnv overrides how long a DeviceServer nobody talks to
// stays up before it exits on its own (a Go duration, e.g. "5m").
const deviceServerIdleTimeoutEnv = "MOBILECLI_DEVICE_SERVER_IDLE_TIMEOUT"

// defaultDeviceServerIdleTimeout matches the daemon's own idle timeout: as long
// as the daemon is being used, the server stays warm.
const defaultDeviceServerIdleTimeout = 30 * time.Minute

// deviceServerStopTimeout bounds how long StopDeviceServersInUse waits for a
// server to be gone after telling it to stop.
const deviceServerStopTimeout = 2 * time.Second

// deviceServersInUse remembers every device whose DeviceServer this process
// has talked to. While a server runs it holds the device's only UiAutomation
// and every other UiAutomator client is killed, so the daemon stops these on
// its way out instead of leaving them behind.
var deviceServersInUse = struct {
	sync.Mutex
	devices map[string]*AndroidDevice
}{devices: map[string]*AndroidDevice{}}

func deviceServerIdleTimeout() time.Duration {
	value := os.Getenv(deviceServerIdleTimeoutEnv)
	if value == "" {
		return defaultDeviceServerIdleTimeout
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		utils.Verbose("ignoring %s=%q: not a positive duration", deviceServerIdleTimeoutEnv, value)
		return defaultDeviceServerIdleTimeout
	}
	return timeout
}

func rememberDeviceServerInUse(d *AndroidDevice) {
	deviceServersInUse.Lock()
	defer deviceServersInUse.Unlock()
	deviceServersInUse.devices[d.getAdbIdentifier()] = d
}

// StopDeviceServersInUse stops the DeviceServer on every device this process
// has used one on, giving the devices' UiAutomation back to other tools. The
// daemon runs it when it shuts down.
func StopDeviceServersInUse() error {
	deviceServersInUse.Lock()
	servers := deviceServersInUse.devices
	deviceServersInUse.devices = map[string]*AndroidDevice{}
	deviceServersInUse.Unlock()

	var errs []error
	for _, d := range servers {
		if err := d.stopDeviceServer(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.getAdbIdentifier(), err))
		}
	}
	return errors.Join(errs...)
}

// stopDeviceServer kills the server on the device and waits for it to be gone,
// so a caller that stops the daemon can hand the device to another UI tool
// right away. The forward and the remembered port go with it.
func (d *AndroidDevice) stopDeviceServer() error {
	utils.Verbose("stopping device server on %s", d.getAdbIdentifier())
	if out, err := d.runAdbCommand("shell", deviceServerKillCommand); err != nil {
		return fmt.Errorf("stop device server: %s: %w", strings.TrimSpace(string(out)), err)
	}

	deadline := time.Now().Add(deviceServerStopTimeout)
	for d.isDeviceServerProcessRunning() {
		if time.Now().After(deadline) {
			return fmt.Errorf("device server still running %s after being told to stop", deviceServerStopTimeout)
		}
		time.Sleep(50 * time.Millisecond)
	}

	d.serverMu.Lock()
	defer d.serverMu.Unlock()
	if d.serverPort != 0 {
		d.removeForward(d.serverPort)
		d.serverPort = 0
	}
	return nil
}

// isDeviceServerProcessRunning asks the device shell; the bracket keeps pgrep
// from matching its own command line.
func (d *AndroidDevice) isDeviceServerProcessRunning() bool {
	_, err := d.runAdbCommand("shell", "pgrep -f '[c]om.mobilenext.mobilecli.DeviceServer'")
	return err == nil
}

// ensureDeviceServerReady returns the port of a DeviceServer that is running,
// forwarded and built from this binary's dex, starting it if needed.
//
// The port is remembered for the lifetime of the device: checking it costs an
// `adb forward --list` process plus two round trips, which is more than most
// calls it guards. serverRequest drops it and comes back here if the server
// turns out to be gone.
func (d *AndroidDevice) ensureDeviceServerReady() (int, error) {
	d.serverMu.Lock()
	defer d.serverMu.Unlock()

	if d.serverPort != 0 {
		return d.serverPort, nil
	}

	port, err := d.startDeviceServer()
	if err != nil {
		return 0, err
	}
	d.serverPort = port
	rememberDeviceServerInUse(d)
	return port, nil
}

// forgetDeviceServer drops the remembered port, so the next call checks the
// server again and restarts it if it is really gone.
func (d *AndroidDevice) forgetDeviceServer() {
	d.serverMu.Lock()
	defer d.serverMu.Unlock()
	d.serverPort = 0
}

// startDeviceServer finds or launches the DeviceServer and returns its host
// port, reusing an existing forward when possible. It's launched detached
// (nohup + &) on the device shell so it outlives this CLI invocation and keeps
// serving subsequent mobilecli calls.
func (d *AndroidDevice) startDeviceServer() (int, error) {
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
	_, _ = d.runAdbCommand("shell", deviceServerKillCommand)

	// nohup+& detaches the server from this adb shell session so it survives
	// after this command returns. The server exits by itself once idle, so a
	// daemon that dies without stopping it does not leave it behind for good.
	launchCmd := fmt.Sprintf("CLASSPATH=%s nohup app_process / %s --idle-timeout-ms=%d >/dev/null 2>&1 &",
		androidDexPath, deviceServerClass, deviceServerIdleTimeout().Milliseconds())
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
	return forwardOnFreePort(d.runAdbCommand, target)
}

// forwardAttempts bounds the retries when adb can't bind the port we picked.
const forwardAttempts = 3

// adbPortTakenMessage is what adb prints when the local port of a forward is in use.
const adbPortTakenMessage = "cannot bind"

// forwardOnFreePort forwards a free local port to target and returns the port. The port
// was free when we probed it, but adb binds it a moment later and, with a remote adb
// server, on another host, so a taken port is retried on a new one.
func forwardOnFreePort(runAdb func(args ...string) ([]byte, error), target string) (int, error) {
	var lastErr error
	for attempt := 0; attempt < forwardAttempts; attempt++ {
		args, port, err := forwardArgs(target)
		if err != nil {
			return 0, err
		}

		out, err := runAdb(args...)
		if err == nil {
			return port, nil
		}

		lastErr = fmt.Errorf("adb forward: %s: %w", strings.TrimSpace(string(out)), err)
		if !strings.Contains(string(out), adbPortTakenMessage) {
			return 0, lastErr
		}
	}
	return 0, lastErr
}

// forwardArgs builds the adb arguments for a forward to target on a free local port that
// we pick. "tcp:0" would let adb pick, but that only works when the adb server runs on this
// machine. With a remote adb server the port adb reports is opened on the other host, and
// tooling that tunnels forwarded ports by their number has no number to tunnel, so every
// agent behind such a forward looks dead.
func forwardArgs(target string) ([]string, int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("find a free local port: %w", err)
	}
	addr, isTCP := listener.Addr().(*net.TCPAddr)
	closeErr := listener.Close()
	if !isTCP {
		return nil, 0, fmt.Errorf("find a free local port: unexpected address %v", listener.Addr())
	}
	port := addr.Port
	if closeErr != nil {
		return nil, 0, fmt.Errorf("release local port %d: %w", port, closeErr)
	}
	return []string{"forward", fmt.Sprintf("tcp:%d", port), target}, port, nil
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
// it if needed. A server that has gone away (device rebooted, another mobilecli
// build restarted it) is set up again once and the call retried, so the
// remembered port never strands a session. A call that timed out is never
// resent: the server may still be running it, and repeating a tap or a line of
// text is worse than reporting the timeout.
func (d *AndroidDevice) serverRequest(method string, params any) (json.RawMessage, error) {
	port, err := d.ensureDeviceServerReady()
	if err != nil {
		return nil, err
	}

	raw, err := agentRequest(port, method, params)
	if !errors.Is(err, errAgentUnreachable) {
		return raw, err
	}

	utils.Verbose("device server on port %d is gone, starting it again", port)
	d.forgetDeviceServer()
	port, startErr := d.ensureDeviceServerReady()
	if startErr != nil {
		return nil, fmt.Errorf("%w (restart failed: %v)", err, startErr)
	}
	return agentRequest(port, method, params)
}

// dumpUiNodes fetches the UI hierarchy from the DeviceServer. Callers fall back
// to uiautomator when it's unavailable.
func (d *AndroidDevice) dumpUiNodes(opts DumpOptions) ([]uiNode, error) {
	startTime := time.Now()

	raw, err := d.serverRequest("device.dump.ui", dumpUiParams{WaitUntilIdle: dumpUiWaitUntilIdleMs, Full: opts.Full})
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
