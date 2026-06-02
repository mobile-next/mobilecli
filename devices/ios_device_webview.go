package devices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/mobile-next/mobilecli/agents"
	iosutil "github.com/mobile-next/mobilecli/devices/ios"
	"github.com/mobile-next/mobilecli/devices/ios/debuggertools"
	"github.com/mobile-next/mobilecli/devices/ios/debugserver"
	"github.com/mobile-next/mobilecli/utils"
)

// ──────────────────────────────────────────────────────────────────────────────
// Real iOS device WebView support.
//
// Unlike the simulator path (ios_webview.go), a real device cannot have a dylib
// injected directly. Instead we attach to the foreground app via the CoreDevice
// debug proxy (go-ios) and, through LLDB, evaluate an ObjC expression that binds
// a TCP socket inside the app and runs a tiny HTTP/JSON-RPC server. We then
// forward a local port to that server and speak the same agent protocol used by
// the simulator and Android paths (agentRequest / WebViewInfo, defined in
// android_webview.go).
//
// Everything device-specific lives in this file so it never conflicts with the
// shared simulator/Android webview code.
// ──────────────────────────────────────────────────────────────────────────────

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

type userApp struct {
	pid      int
	bundleID string
	teamID   string
}

// userApps returns all currently running user-installed apps (PID + bundle ID),
// using installationproxy + instruments, with no WDA dependency.
func (d *IOSDevice) userApps(device goios.DeviceEntry) ([]userApp, error) {
	utils.Verbose("connecting to installationproxy")
	svc, err := installationproxy.New(device)
	if err != nil {
		return nil, fmt.Errorf("installationproxy: %w", err)
	}
	defer svc.Close()

	utils.Verbose("browsing user apps")
	apps, err := svc.BrowseUserApps()
	if err != nil {
		return nil, fmt.Errorf("browse user apps: %w", err)
	}
	utils.Verbose("found %d installed user apps", len(apps))
	execToBundleID := map[string]string{}
	for _, app := range apps {
		execToBundleID[app.CFBundleExecutable()] = app.CFBundleIdentifier()
	}

	utils.Verbose("connecting to instruments device info service")
	infoSvc, err := instruments.NewDeviceInfoService(device)
	if err != nil {
		return nil, fmt.Errorf("device info service: %w", err)
	}
	defer infoSvc.Close()

	utils.Verbose("fetching process list")
	processes, err := infoSvc.ProcessList()
	if err != nil {
		return nil, fmt.Errorf("process list: %w", err)
	}
	utils.Verbose("got %d processes", len(processes))

	// also build a map from bundleID to teamIdentifier
	bundleToTeam := map[string]string{}
	for _, app := range apps {
		if tid, ok := app["TeamIdentifier"].(string); ok {
			bundleToTeam[app.CFBundleIdentifier()] = tid
		}
	}

	var result []userApp
	for _, p := range processes {
		if bid, ok := execToBundleID[p.Name]; ok {
			result = append(result, userApp{pid: int(p.Pid), bundleID: bid, teamID: bundleToTeam[bid]})
		}
	}
	utils.Verbose("found %d running user apps", len(result))
	return result, nil
}

// findForegroundApp finds the foreground user app by attaching to each candidate
// via the CoreDevice debug proxy and checking UIApplicationState via ObjC runtime.
func (d *IOSDevice) findForegroundApp(device goios.DeviceEntry, apps []userApp) (*userApp, error) {
	if !device.SupportsRsd() {
		return nil, fmt.Errorf("device does not support RSD")
	}
	proxyPort := device.Rsd.GetPort("com.apple.internal.dt.remote.debugproxy")
	if proxyPort == 0 {
		return nil, fmt.Errorf("com.apple.internal.dt.remote.debugproxy not in RSD")
	}
	utils.Verbose("debug proxy port: %d", proxyPort)

	for i := range apps {
		app := &apps[i]
		utils.Verbose("checking app %s (pid %d)", app.bundleID, app.pid)
		conn, err := goios.ConnectTUNDevice(device.Address, proxyPort, device)
		if err != nil {
			utils.Verbose("connect to debug proxy for %s: %v", app.bundleID, err)
			continue
		}
		gdb := debugserver.NewGDBServer(conn)
		utils.Verbose("attaching to pid %d", app.pid)
		resp, err := gdb.Request(fmt.Sprintf("vAttach;%x", app.pid))
		if err != nil || !strings.HasPrefix(resp, "T") {
			utils.Verbose("attach to pid %d failed: err=%v resp=%q", app.pid, err, resp)
			conn.Close()
			continue
		}
		utils.Verbose("attached to pid %d, checking UIApplicationState", app.pid)
		rt, err := debuggertools.NewObjCRuntime(gdb)
		if err != nil {
			utils.Verbose("ObjCRuntime for pid %d: %v", app.pid, err)
			gdb.Request(fmt.Sprintf("D;%x", app.pid)) //nolint:errcheck
			conn.Close()
			continue
		}
		appInst, err := rt.ClassCall("UIApplication", "sharedApplication")
		var state uint64
		if err == nil {
			state, _ = rt.Call(appInst, "applicationState")
		}
		rt.Cleanup()
		gdb.Request(fmt.Sprintf("D;%x", app.pid)) //nolint:errcheck
		conn.Close()
		utils.Verbose("pid %d (%s) applicationState=%d", app.pid, app.bundleID, state)
		if err == nil && state == 0 {
			utils.Verbose("foreground app: %s (pid %d)", app.bundleID, app.pid)
			return app, nil
		}
	}
	return nil, fmt.Errorf("no foreground user app found — is an app open?")
}

// iosDeviceAgentPort is the fixed device-side TCP port the injected agent binds
// (see agents/ios-real/agent.m). A fixed port lets the reuse fast-path find an
// already-running agent without scanning or persisting state between runs.
const iosDeviceAgentPort = 12008

var deviceAgentPortRE = regexp.MustCompile(`\$\d+\s*=\s*(\d+)`)

// startLLDBProxy pre-attaches to pid on the device via the debug proxy, then
// starts a local TCP listener for LLDB. Pre-attaching before listening ensures
// the proxy can respond to LLDB's handshake immediately upon connection.
// Returns the local port and a stop function.
func startLLDBProxy(device goios.DeviceEntry, proxyPort, pid int) (int, func(), error) {
	utils.Verbose("lldb-proxy: connecting to device debug proxy port %d", proxyPort)
	devConn, err := goios.ConnectTUNDevice(device.Address, proxyPort, device)
	if err != nil {
		return 0, nil, fmt.Errorf("lldb-proxy: connect to device: %w", err)
	}

	devGDB := debugserver.NewGDBServer(devConn)
	utils.Verbose("lldb-proxy: pre-attaching to pid %d", pid)
	stopReply, err := devGDB.Request(fmt.Sprintf("vAttach;%x", pid))
	if err != nil || !strings.HasPrefix(stopReply, "T") {
		devConn.Close()
		return 0, nil, fmt.Errorf("lldb-proxy: vAttach pid %d: err=%v resp=%q", pid, err, stopReply)
	}
	utils.Verbose("lldb-proxy: pre-attached, stop=%q", stopReply)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		devConn.Close()
		return 0, nil, fmt.Errorf("listen for lldb proxy: %w", err)
	}
	localPort := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer ln.Close()
		defer devConn.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		lldbProxyConn(conn, devGDB, pid)
	}()
	return localPort, func() { ln.Close() }, nil
}

// lldbProxyConn is a GDB RSP bridge between LLDB and an already-attached
// device debugserver. Handles negotiation packets locally, forwards all others
// packet-by-packet with ack-mode translation (LLDB: no-ack; device: ack).
func lldbProxyConn(c net.Conn, devGDB *debugserver.GDBServer, pid int) {
	defer c.Close()

	// debugserver sends '+' immediately upon accepting a connection;
	// LLDB waits for this before sending the first packet.
	c.Write([]byte("+")) //nolint:errcheck

	noAck := false

	gdbChecksum := func(pkt string) byte {
		var sum byte
		for i := 0; i < len(pkt); i++ {
			sum += pkt[i]
		}
		return sum
	}

	sendToLLDB := func(pkt string) {
		ck := gdbChecksum(pkt)
		var s string
		if !noAck {
			s = "+"
		}
		s += fmt.Sprintf("$%s#%02x", pkt, ck)
		c.Write([]byte(s)) //nolint:errcheck
	}

	recvFromLLDB := func() (string, error) {
		buf := make([]byte, 1)
		for {
			if _, err := io.ReadFull(c, buf); err != nil {
				return "", err
			}
			if buf[0] == '$' {
				break
			}
		}
		var pkt strings.Builder
		for {
			if _, err := io.ReadFull(c, buf); err != nil {
				return "", err
			}
			if buf[0] == '#' {
				break
			}
			pkt.WriteByte(buf[0])
		}
		cksumBuf := make([]byte, 2)
		if _, err := io.ReadFull(c, cksumBuf); err != nil {
			return "", err
		}
		return pkt.String(), nil
	}

	for {
		pkt, err := recvFromLLDB()
		if err != nil {
			return
		}
		utils.Verbose("lldb-proxy ← LLDB: %.300s", pkt)

		// switchToNoAck is set by QStartNoAckMode and applied AFTER sendToLLDB
		// so the OK response goes out in ack mode (with '+') as LLDB expects.
		switchToNoAck := false
		var reply string
		switch {
		case pkt == "QStartNoAckMode":
			reply = "OK"
			switchToNoAck = true

		case strings.HasPrefix(pkt, "qSupported"):
			reply = "PacketSize=65536;vContSupported+"

		case pkt == "QThreadSuffixSupported",
			pkt == "QListThreadsInStopReply",
			pkt == "qVAttachOrWaitSupported",
			pkt == "QEnableErrorStrings":
			reply = "OK"

		case strings.HasPrefix(pkt, "vCont?"):
			reply = "vCont;c;C;s;S"

		case pkt == "k":
			// LLDB wants to kill — detach instead so the app keeps running
			devGDB.Request(fmt.Sprintf("D;%x", pid)) //nolint:errcheck
			return

		case strings.HasPrefix(pkt, "D"):
			devReply, _ := devGDB.Request(pkt)
			utils.Verbose("lldb-proxy → LLDB (detach): %d bytes", len(devReply))
			sendToLLDB(devReply)
			return

		default:
			// forward to device (devGDB uses ack mode: sends "+$pkt#XX")
			devReply, err := devGDB.Request(pkt)
			if err != nil {
				utils.Verbose("lldb-proxy: device error for %q: %v", pkt[:min(len(pkt), 40)], err)
				return
			}
			reply = devReply
		}

		utils.Verbose("lldb-proxy → LLDB: %d bytes", len(reply))
		sendToLLDB(reply)
		if switchToNoAck {
			noAck = true
		}
	}
}

// injectServerViaLLDB connects LLDB to the proxy (which has already attached to
// the target process), evaluates iosDeviceAgentExpr to start a persistent HTTP
// server inside the app, and returns the device-side TCP port.
func injectServerViaLLDB(localProxyPort int) (int, error) {
	const lldbTimeout = 120 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), lldbTimeout)
	defer cancel()

	utils.Verbose("running LLDB (timeout %s)", lldbTimeout)
	cmd := exec.CommandContext(ctx, "lldb",
		"-o", "settings set target.process.memory-cache-line-size 16384",
		"-o", "platform select remote-ios",
		"-o", fmt.Sprintf("process connect connect://localhost:%d", localProxyPort),
		// The leading newline after "--" is required: it puts LLDB into multi-line
		// expression mode so the whole agent source is treated as one expression
		// (without it, LLDB runs only the first line and parses the rest as commands).
		"-o", "expr -l objc -- \n"+agents.IOSRealDeviceWebViewAgent,
		"-o", "detach",
		"-o", "quit",
	)
	out, err := cmd.CombinedOutput()
	utils.Verbose("LLDB finished (err=%v), output:\n%s", err, out)
	if err != nil {
		return 0, fmt.Errorf("lldb: %w\noutput:\n%s", err, out)
	}

	for _, line := range strings.Split(string(out), "\n") {
		m := deviceAgentPortRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port, err := strconv.Atoi(m[1])
		if err == nil && port == iosDeviceAgentPort {
			return port, nil
		}
	}
	return 0, fmt.Errorf("could not parse port from lldb output:\n%s", out)
}

// freeLocalPort asks the kernel for an unused local TCP port (bind :0, read it
// back, release it). Only the device-side agent port is fixed
// (iosDeviceAgentPort, for cross-run reuse discovery); the local end of the
// forward is ephemeral and lives only for this process, so we let the OS pick
// it. (go-ios's forward.Forward rejects a literal port 0, so we grab one here.)
func freeLocalPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
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

func (d *IOSDevice) ensureIOSDeviceAgentReady() (int, error) {
	// fast path: reuse the forwarded port we set up for this device previously
	if port, ok := cachedDeviceAgentPort(d.Udid); ok && isAgentReady(port) {
		utils.Verbose("reusing cached agent port %d", port)
		return port, nil
	}

	if err := d.startTunnel(); err != nil {
		return 0, fmt.Errorf("start tunnel: %w", err)
	}

	// fast path: an agent injected by a previous run may still be alive inside
	// the app. Probe the device port range and reuse it, skipping the costly
	// LLDB injection entirely.
	if port, ok := d.findRunningDeviceAgent(); ok {
		setCachedDeviceAgentPort(d.Udid, port)
		return port, nil
	}

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

	utils.Verbose("listing running user apps")
	apps, err := d.userApps(device)
	if err != nil {
		return 0, err
	}
	if len(apps) == 0 {
		return 0, fmt.Errorf("no user app running — open an app first")
	}

	utils.Verbose("finding foreground app among %d candidates", len(apps))
	foreground, err := d.findForegroundApp(device, apps)
	if err != nil {
		return 0, err
	}

	utils.Verbose("injecting agent into %s (pid %d) via LLDB", foreground.bundleID, foreground.pid)
	// start a local TCP proxy so LLDB can reach the device debug proxy
	lldbProxyPort, cancelProxy, err := startLLDBProxy(device, proxyPort, foreground.pid)
	if err != nil {
		return 0, fmt.Errorf("start lldb proxy: %w", err)
	}
	defer cancelProxy()
	utils.Verbose("LLDB proxy listening on localhost:%d", lldbProxyPort)

	devicePort, err := injectServerViaLLDB(lldbProxyPort)
	if err != nil {
		return 0, fmt.Errorf("inject server via lldb: %w", err)
	}
	utils.Verbose("agent started on device port %d", devicePort)

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
	setCachedDeviceAgentPort(d.Udid, localPort)
	return localPort, nil
}

// ── Public WebViewable implementation for real iOS devices ────────────────────

func (d *IOSDevice) ListWebViews() ([]WebViewInfo, error) {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return nil, err
	}
	result, err := agentRequest(port, "device.webview.list", nil)
	if err != nil {
		setCachedDeviceAgentPort(d.Udid, 0)
		return nil, err
	}
	var raw []struct {
		ID      string         `json:"id"`
		URL     string         `json:"url"`
		Title   string         `json:"title"`
		Bounds  map[string]any `json:"bounds"`
		Visible bool           `json:"visible"`
	}
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, fmt.Errorf("parse webview list: %w", err)
	}
	webviews := make([]WebViewInfo, len(raw))
	for i, wv := range raw {
		webviews[i] = WebViewInfo{ID: wv.ID, URL: wv.URL, Title: wv.Title, Bounds: wv.Bounds, IsVisible: wv.Visible}
	}
	return webviews, nil
}

func (d *IOSDevice) WebViewGoto(webviewID, url string) error {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goto", map[string]any{"id": webviewID, "url": url})
	return err
}

func (d *IOSDevice) WebViewReload(webviewID string) error {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.reload", map[string]any{"id": webviewID})
	return err
}

func (d *IOSDevice) WebViewGoBack(webviewID string) error {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goBack", map[string]any{"id": webviewID})
	return err
}

func (d *IOSDevice) WebViewGoForward(webviewID string) error {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goForward", map[string]any{"id": webviewID})
	return err
}

func (d *IOSDevice) WebViewContent(webviewID string) (string, error) {
	result, err := d.WebViewEvaluate(webviewID, "return document.documentElement.outerHTML", nil)
	if err != nil {
		return "", err
	}
	s, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T", result)
	}
	return s, nil
}

func (d *IOSDevice) WebViewEvaluate(webviewID, expression string, args []any) (any, error) {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"id":         webviewID,
		"expression": ensureReturnExpression(expression),
	}
	if len(args) > 0 {
		params["args"] = args
	}
	raw, err := agentRequest(port, "device.webview.evaluate", params)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Result any `json:"result"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse evaluate result: %w", err)
	}
	return wrapper.Result, nil
}

func (d *IOSDevice) WebViewWaitForLoadState(webviewID, state string, timeoutMs int) error {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return err
	}
	if timeoutMs <= 0 {
		timeoutMs = 30_000
	}
	_, err = agentRequestWithTimeout(port, "device.webview.waitForLoadState", map[string]any{
		"id":      webviewID,
		"state":   state,
		"timeout": timeoutMs,
	}, time.Duration(timeoutMs+5000)*time.Millisecond)
	return err
}
