package devices

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	iosutil "github.com/mobile-next/mobilecli/devices/ios"
	"github.com/mobile-next/mobilecli/utils"
)

// ──────────────────────────────────────────────────────────────────────────────
// Real iOS device agent: the reusable foundation that injects an HTTP/JSON-RPC
// agent into the foreground app and keeps a forwarded connection to it.
//
// A real device cannot have a dylib injected directly. Instead we ask WDA for
// the foreground app (bundleID + pid), attach to it via the CoreDevice debug
// proxy (go-ios) and, through LLDB, evaluate an ObjC expression that binds a TCP
// socket inside the app and runs a tiny HTTP/JSON-RPC server (see
// agents/ios-real/agent.m). We then forward a local port to that server.
//
// Feature code (webview today; view-tree, network capture, etc. later) talks to
// the agent only through agentCall — it never touches LLDB/proxy details.
// ──────────────────────────────────────────────────────────────────────────────

// iosDeviceAgentPort is the fixed device-side TCP port the injected agent binds
// (see agents/ios-real/agent.m). A fixed port lets the reuse fast-path find an
// already-running agent without scanning or persisting state between runs.
const iosDeviceAgentPort = 12008

// deviceAgentPortCache maps device UDID → local TCP port of its injected agent.
// This is intentionally separate from the simulator's (udid, bundleID) cache in
// ios_webview.go: a real device runs a single injected agent per device.
var (
	deviceAgentPortCache   = map[string]int{}
	deviceAgentPortCacheMu sync.Mutex
)

func cachedDeviceAgentPort(udid string) (int, bool) {
	deviceAgentPortCacheMu.Lock()
	defer deviceAgentPortCacheMu.Unlock()
	port, ok := deviceAgentPortCache[udid]
	return port, ok
}

func setCachedDeviceAgentPort(udid string, port int) {
	deviceAgentPortCacheMu.Lock()
	defer deviceAgentPortCacheMu.Unlock()
	deviceAgentPortCache[udid] = port
}

// agentCall ensures the agent is ready and makes a JSON-RPC call to it. This is
// the single seam every device-agent feature uses; it also drops the cached
// port on failure so a dead agent is re-injected on the next call.
func (d *IOSDevice) agentCall(method string, params any) (json.RawMessage, error) {
	return d.agentCallWithTimeout(method, params, defaultAgentTimeout)
}

func (d *IOSDevice) agentCallWithTimeout(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return nil, err
	}
	raw, err := agentRequestWithTimeout(port, method, params, timeout)
	if err != nil {
		// the agent may have died; drop the cached port so we re-inject next time
		setCachedDeviceAgentPort(d.Udid, 0)
	}
	return raw, err
}

// ensureIOSDeviceAgentReady returns a local port connected to a live agent,
// trying (1) the in-process cache, (2) an agent left running by a previous run,
// then (3) a fresh injection.
func (d *IOSDevice) ensureIOSDeviceAgentReady() (int, error) {
	if port, ok := cachedDeviceAgentPort(d.Udid); ok && isAgentReady(port) {
		utils.Verbose("reusing cached agent port %d", port)
		return port, nil
	}

	if err := d.startTunnel(); err != nil {
		return 0, fmt.Errorf("start tunnel: %w", err)
	}

	if port, ok := d.findRunningDeviceAgent(); ok {
		setCachedDeviceAgentPort(d.Udid, port)
		return port, nil
	}

	port, err := d.injectFreshAgent()
	if err != nil {
		return 0, err
	}
	setCachedDeviceAgentPort(d.Udid, port)
	return port, nil
}

// findRunningDeviceAgent checks whether an agent from a previous injection is
// still alive on the fixed device port (the injected server persists inside the
// app after LLDB detaches). It forwards a local port to iosDeviceAgentPort and
// checks whether the HTTP/JSON-RPC agent answers, letting the caller skip LLDB
// injection.
//
// Caveat: this reuses whatever agent is alive on that device port. If the
// foreground app changed since the last injection but the previous app is still
// running, this may talk to that previous app's agent.
func (d *IOSDevice) findRunningDeviceAgent() (int, bool) {
	localPort, err := freeLocalPort()
	if err != nil {
		return 0, false
	}
	pf := iosutil.NewPortForwarder(d.Udid)
	if err := pf.Forward(localPort, iosDeviceAgentPort); err != nil {
		return 0, false
	}
	if isAgentReady(localPort) {
		utils.Verbose("reusing running agent on device port %d (local %d)", iosDeviceAgentPort, localPort)
		return localPort, true
	}
	pf.Stop() //nolint:errcheck
	return 0, false
}

// injectFreshAgent performs the full (slow) path: resolve the foreground app via
// WDA, inject the agent into it over LLDB, forward an ephemeral local port to
// the agent, and wait for it to answer. Returns the ready local port.
func (d *IOSDevice) injectFreshAgent() (int, error) {
	utils.Verbose("getting enhanced device info")
	device, err := d.getEnhancedDevice()
	if err != nil {
		return 0, fmt.Errorf("get enhanced device: %w", err)
	}
	proxyPort, err := iosDeviceDebugProxyPort(device)
	if err != nil {
		return 0, err
	}
	utils.Verbose("debug proxy port from RSD: %d", proxyPort)

	// ensure the devicekit/WDA agent is up (tunnel + :8100 forward + launch);
	// idempotent, and required for the foreground-app lookup below.
	if err := d.StartAgent(StartAgentConfig{}); err != nil {
		return 0, fmt.Errorf("start device agent (WDA): %w", err)
	}

	utils.Verbose("getting foreground app via WDA")
	activeApp, err := d.deviceKitClient.GetActiveAppInfo()
	if err != nil {
		return 0, fmt.Errorf("get foreground app: %w", err)
	}
	if activeApp.ProcessID == 0 {
		return 0, fmt.Errorf("no foreground app (pid 0) — open an app first")
	}

	utils.Verbose("mapping agent dylib into %s (pid %d)", activeApp.BundleID, activeApp.ProcessID)
	devicePort, err := injectAgentNative(device, proxyPort, activeApp.ProcessID)
	if err != nil {
		return 0, fmt.Errorf("inject agent: %w", err)
	}
	utils.Verbose("agent started on device port %d", devicePort)

	return d.forwardToAgent(devicePort)
}

// forwardToAgent forwards an ephemeral local port to the agent's device port and
// waits for the agent to start answering. Returns the ready local port.
func (d *IOSDevice) forwardToAgent(devicePort int) (int, error) {
	localPort, err := freeLocalPort()
	if err != nil {
		return 0, err
	}
	utils.Verbose("forwarding localhost:%d -> device:%d", localPort, devicePort)
	pf := iosutil.NewPortForwarder(d.Udid)
	if err := pf.Forward(localPort, devicePort); err != nil {
		return 0, fmt.Errorf("port forward %d->%d: %w", localPort, devicePort, err)
	}

	utils.Verbose("waiting for agent to respond on port %d", localPort)
	deadline := time.Now().Add(3 * time.Second)
	for !isAgentReady(localPort) {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("iOS device agent did not respond within 3s")
		}
		time.Sleep(100 * time.Millisecond)
	}
	utils.Verbose("agent ready on port %d", localPort)
	return localPort, nil
}

func iosDeviceDebugProxyPort(device goios.DeviceEntry) (int, error) {
	if !device.SupportsRsd() {
		return 0, fmt.Errorf("device does not support RSD — enable developer mode")
	}
	p := device.Rsd.GetPort("com.apple.internal.dt.remote.debugproxy")
	if p == 0 {
		return 0, fmt.Errorf("com.apple.internal.dt.remote.debugproxy not in RSD")
	}
	return p, nil
}

// freeLocalPort asks the kernel for an unused local TCP port (bind :0, read it
// back, release it). Only the device-side agent port is fixed
// (iosDeviceAgentPort, for cross-run reuse discovery); the local end of the
// forward is ephemeral and lives only for this process, so we let the OS pick
// it. (go-ios's forward.Forward rejects a literal port 0, so we grab one here.)
func freeLocalPort() (int, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = ln.Close() }()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("listener address is %T, not TCP", ln.Addr())
	}
	return addr.Port, nil
}
