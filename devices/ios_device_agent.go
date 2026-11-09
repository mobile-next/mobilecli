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
// A real device cannot have a dylib loaded with dlopen. Instead we ask the
// DeviceKit agent for the foreground app (bundleID + pid), attach to it over the
// CoreDevice debug proxy, and map the compiled agent (agents/ios) into it (see
// devices/ios/machomap). We then forward a local port to the agent's server.
//
// Feature code (webview today; view-tree, network capture, etc. later) talks to
// the agent only through agentCall — it never touches LLDB/proxy details.
// ──────────────────────────────────────────────────────────────────────────────

// deviceAgentKey identifies one injected agent. The agent lives inside one app
// process, so the cache is per process: another app must not reach this agent,
// and a relaunched app gets a new pid.
type deviceAgentKey struct {
	udid string
	pid  int
}

// injectedAgent is an agent living in an app process. The device port stays valid
// for the life of the process, so a lost forward can be replaced without mapping
// the agent again, which a running app refuses.
type injectedAgent struct {
	bundleID   string
	devicePort int
	localPort  int
	forwarder  *iosutil.PortForwarder
}

var (
	deviceAgentCache = map[deviceAgentKey]*injectedAgent{}
	deviceAgentLocks = map[string]*sync.Mutex{}
	deviceAgentMu    sync.Mutex
)

func cachedDeviceAgent(key deviceAgentKey) (*injectedAgent, bool) {
	deviceAgentMu.Lock()
	defer deviceAgentMu.Unlock()
	agent, ok := deviceAgentCache[key]
	return agent, ok
}

// cachedAgentForApp ignores an agent left by another app: iOS can give a dead
// app's pid to a new app, and the old agent died with the old process.
func cachedAgentForApp(key deviceAgentKey, bundleID string) (*injectedAgent, bool) {
	agent, ok := cachedDeviceAgent(key)
	if !ok || agent.bundleID != bundleID {
		return nil, false
	}
	return agent, true
}

// rememberInjectedAgent caches a new agent and drops the entries it replaces: the
// app's earlier processes, whose agents died with them, and any entry on this pid.
func rememberInjectedAgent(key deviceAgentKey, agent *injectedAgent) {
	deviceAgentMu.Lock()
	defer deviceAgentMu.Unlock()
	for otherKey, other := range deviceAgentCache {
		if otherKey.udid != key.udid || (other.bundleID != agent.bundleID && otherKey != key) {
			continue
		}
		if other.forwarder != nil {
			_ = other.forwarder.Stop()
		}
		delete(deviceAgentCache, otherKey)
	}
	deviceAgentCache[key] = agent
}

// deviceAgentLock serializes agent setup per device: a second caller must wait and
// reuse the agent, because debugserver allows one attach and refuses a second map.
func deviceAgentLock(udid string) *sync.Mutex {
	deviceAgentMu.Lock()
	defer deviceAgentMu.Unlock()
	lock, ok := deviceAgentLocks[udid]
	if !ok {
		lock = &sync.Mutex{}
		deviceAgentLocks[udid] = lock
	}
	return lock
}

// agentCall ensures the agent is ready and makes a JSON-RPC call to it. This is
// the single seam every device-agent feature uses.
func (d *IOSDevice) agentCall(method string, params any) (json.RawMessage, error) {
	return d.agentCallWithTimeout(method, params, defaultAgentTimeout)
}

func (d *IOSDevice) agentCallWithTimeout(method string, params any, timeout time.Duration) (json.RawMessage, error) {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return nil, err
	}
	return agentRequestWithTimeout(port, method, params, timeout)
}

// ensureIOSDeviceAgentReady returns a local port connected to a live agent in
// the foreground app, injecting one if that app process has none yet.
func (d *IOSDevice) ensureIOSDeviceAgentReady() (int, error) {
	if err := d.startTunnel(); err != nil {
		return 0, fmt.Errorf("start tunnel: %w", err)
	}

	// ensure the DeviceKit agent is up (tunnel + :8100 forward + launch);
	// idempotent, and required for the foreground-app lookup below.
	if err := d.StartAgent(StartAgentConfig{}); err != nil {
		return 0, fmt.Errorf("start device agent: %w", err)
	}

	activeApp, err := d.deviceKitClient.GetActiveAppIdentity()
	if err != nil {
		return 0, fmt.Errorf("get foreground app: %w", err)
	}
	if activeApp.ProcessID == 0 {
		return 0, fmt.Errorf("no foreground app (pid 0) — open an app first")
	}

	lock := deviceAgentLock(d.Udid)
	lock.Lock()
	defer lock.Unlock()

	key := deviceAgentKey{udid: d.Udid, pid: activeApp.ProcessID}
	agent, ok := cachedAgentForApp(key, activeApp.BundleID)
	if ok && isAgentReady(agent.localPort) {
		utils.Verbose("reusing cached agent port %d", agent.localPort)
		return agent.localPort, nil
	}
	if !ok {
		devicePort, err := d.injectFreshAgent(activeApp.BundleID, activeApp.ProcessID)
		if err != nil {
			return 0, err
		}
		agent = &injectedAgent{bundleID: activeApp.BundleID, devicePort: devicePort}
		rememberInjectedAgent(key, agent)
	}
	return d.connectToAgent(agent)
}

// injectFreshAgent performs the slow path: map the agent into the app over the
// debug proxy. Returns the device port the agent's server bound.
func (d *IOSDevice) injectFreshAgent(bundleID string, pid int) (int, error) {
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

	utils.Verbose("mapping agent dylib into %s (pid %d)", bundleID, pid)
	devicePort, err := injectAgentNative(device, proxyPort, pid)
	if err != nil {
		return 0, fmt.Errorf("inject agent: %w", err)
	}
	utils.Verbose("agent started on device port %d", devicePort)
	return devicePort, nil
}

// connectToAgent replaces the agent's forward with a new ephemeral local port and
// waits for the agent to answer on it. Returns the ready local port.
func (d *IOSDevice) connectToAgent(agent *injectedAgent) (int, error) {
	if agent.forwarder != nil {
		_ = agent.forwarder.Stop()
		agent.forwarder = nil
	}

	localPort, err := freeLocalPort()
	if err != nil {
		return 0, err
	}
	utils.Verbose("forwarding localhost:%d -> device:%d", localPort, agent.devicePort)
	pf := iosutil.NewPortForwarder(d.Udid)
	if err := pf.Forward(localPort, agent.devicePort); err != nil {
		return 0, fmt.Errorf("port forward %d->%d: %w", localPort, agent.devicePort, err)
	}

	utils.Verbose("waiting for agent to respond on port %d", localPort)
	deadline := time.Now().Add(3 * time.Second)
	for !isAgentReady(localPort) {
		if time.Now().After(deadline) {
			_ = pf.Stop()
			return 0, fmt.Errorf("iOS device agent did not respond within 3s")
		}
		time.Sleep(100 * time.Millisecond)
	}
	utils.Verbose("agent ready on port %d", localPort)
	agent.localPort = localPort
	agent.forwarder = pf
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
// back, release it). go-ios's forward.Forward rejects a literal port 0.
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
