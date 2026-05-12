package devices

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/mobile-next/mobilecli/agents"
	iosutil "github.com/mobile-next/mobilecli/devices/ios"
)

// findSimulatorForegroundApp searches the Mac process list for an app process
// running inside the given simulator and returns its PID and bundle ID.
func findSimulatorForegroundApp(udid string) (pid int, bundleID string, err error) {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		return 0, "", fmt.Errorf("ps aux: %w", err)
	}

	// match lines for app binaries inside this simulator's Bundle directory
	pattern := fmt.Sprintf(`CoreSimulator/Devices/%s/data/Containers/Bundle`, udid)
	var candidates []struct {
		pid  int
		path string
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, pattern) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		p, parseErr := strconv.Atoi(fields[1])
		if parseErr != nil {
			continue
		}
		// fields[10] is the executable path
		candidates = append(candidates, struct {
			pid  int
			path string
		}{p, fields[10]})
	}

	if len(candidates) == 0 {
		return 0, "", fmt.Errorf("no app process found in simulator %s — is an app running?", udid)
	}
	if len(candidates) > 1 {
		// pick the most recently listed (last in ps output) as a heuristic for foreground
		// in practice most simulator sessions have a single app
	}
	candidate := candidates[len(candidates)-1]

	// extract the .app bundle directory from the executable path
	appBundlePath := appBundleFromExecPath(candidate.path)
	if appBundlePath == "" {
		return 0, "", fmt.Errorf("could not locate .app bundle from path %q", candidate.path)
	}

	bid, err := bundleIDFromInfoPlist(filepath.Join(appBundlePath, "Info.plist"))
	if err != nil {
		return 0, "", fmt.Errorf("read bundle ID: %w", err)
	}

	return candidate.pid, bid, nil
}

// appBundleFromExecPath returns the .app directory containing the executable.
// e.g. ".../playground.app/playground" → ".../playground.app"
func appBundleFromExecPath(execPath string) string {
	re := regexp.MustCompile(`(.*\.app)/`)
	m := re.FindStringSubmatch(execPath)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// bundleIDFromInfoPlist reads CFBundleIdentifier from an Info.plist using
// the macOS `defaults read` command, which handles both XML and binary plists.
func bundleIDFromInfoPlist(plistPath string) (string, error) {
	out, err := exec.Command("defaults", "read", plistPath, "CFBundleIdentifier").Output()
	if err != nil {
		return "", fmt.Errorf("defaults read %s: %w", plistPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// writeIOSAgentDylib writes the embedded simulator dylib to a temp file and
// returns the path. The caller is responsible for removing the file.
func writeIOSAgentDylib() (string, error) {
	f, err := os.CreateTemp("", "mobilecli-agent-*.dylib")
	if err != nil {
		return "", fmt.Errorf("create temp dylib: %w", err)
	}
	if _, err := f.Write(agents.IOSAgentSimDylib); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write dylib: %w", err)
	}
	f.Close()
	return f.Name(), nil
}

var portFromLLDB = regexp.MustCompile(`\$\d+\s*=\s*(\d+)`)

// injectIOSAgent attaches lldb to the given PID, loads the dylib, reads the
// bound port via mobilecli_get_port(), then detaches. Returns the port.
func injectIOSAgent(pid int, dylibPath string) (int, error) {
	cmd := exec.Command("lldb",
		"-p", strconv.Itoa(pid),
		"-o", fmt.Sprintf("expr (void*)dlopen(%q, 2)", dylibPath),
		"-o", "expr (int)mobilecli_get_port()",
		"-o", "detach",
		"-o", "quit",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("lldb: %w\noutput:\n%s", err, out)
	}

	// find the port in a line like: (int) $1 = 27042
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "mobilecli_get_port") {
			continue
		}
		m := portFromLLDB.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port, err := strconv.Atoi(m[1])
		if err != nil || port == 0 {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("could not parse port from lldb output:\n%s", out)
}

// ensureIOSAgentReady ensures the iOS agent is running inside the simulator
// and returns the local TCP port to connect to.
func (s *SimulatorDevice) ensureIOSAgentReady() (int, error) {
	// fast path: agent already running from a previous call
	for port := 27042; port <= 27051; port++ {
		if isAgentReady(port) {
			return port, nil
		}
	}

	pid, _, err := findSimulatorForegroundApp(s.UDID)
	if err != nil {
		return 0, err
	}

	dylibPath, err := writeIOSAgentDylib()
	if err != nil {
		return 0, err
	}
	defer os.Remove(dylibPath)

	port, err := injectIOSAgent(pid, dylibPath)
	if err != nil {
		return 0, fmt.Errorf("inject agent: %w", err)
	}

	// agent binds synchronously before dlopen returns, but give it a moment
	deadline := time.Now().Add(3 * time.Second)
	for !isAgentReady(port) {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("iOS agent did not respond on port %d within 3s", port)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return port, nil
}

func (s *SimulatorDevice) webViewAction(wvID, method string) error {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, method, map[string]any{"id": wvID})
	return err
}

// WebViewReload reloads the page in the given webview.
func (s *SimulatorDevice) WebViewReload(wvID string) error {
	return s.webViewAction(wvID, "device.webview.reload")
}

// WebViewGoBack navigates the given webview back in history.
func (s *SimulatorDevice) WebViewGoBack(wvID string) error {
	return s.webViewAction(wvID, "device.webview.goBack")
}

// WebViewGoForward navigates the given webview forward in history.
func (s *SimulatorDevice) WebViewGoForward(wvID string) error {
	return s.webViewAction(wvID, "device.webview.goForward")
}

// WebViewContent returns the full outer HTML of the page in the given webview.
func (s *SimulatorDevice) WebViewContent(wvID string) (string, error) {
	result, err := s.WebViewEvaluate(wvID, "return document.documentElement.outerHTML", nil)
	if err != nil {
		return "", err
	}
	content, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T", result)
	}
	return content, nil
}

// WebViewWaitForLoadState blocks until the webview reaches the given load state.
// timeoutMs of 0 uses the agent's default (30s).
func (s *SimulatorDevice) WebViewWaitForLoadState(wvID, state string, timeoutMs int) error {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return err
	}
	const agentDefaultMs = 30_000
	waitMs := agentDefaultMs
	if timeoutMs > 0 {
		waitMs = timeoutMs
	}
	params := map[string]any{"id": wvID, "timeout": waitMs}
	if state != "" {
		params["state"] = state
	}
	httpTimeout := time.Duration(waitMs)*time.Millisecond + 5*time.Second
	_, err = agentRequestWithTimeout(port, "device.webview.waitForLoadState", params, httpTimeout)
	return err
}

// WebViewGoto navigates the webview identified by wvID to url.
func (s *SimulatorDevice) WebViewGoto(wvID, url string) error {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goto", map[string]any{"id": wvID, "url": url})
	return err
}

// WebViewEvaluate runs expression in the webview and returns the JS result value.
func (s *SimulatorDevice) WebViewEvaluate(wvID, expression string, args []any) (any, error) {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"id":         wvID,
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

// ListWebViews returns all embedded WKWebViews found in the foreground simulator app.
func (s *SimulatorDevice) ListWebViews() ([]WebViewInfo, error) {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return nil, err
	}

	result, err := agentRequest(port, "device.webview.list", nil)
	if err != nil {
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
		webviews[i] = WebViewInfo{
			ID:        wv.ID,
			URL:       wv.URL,
			Title:     wv.Title,
			Bounds:    wv.Bounds,
			IsVisible: wv.Visible,
		}
	}
	return webviews, nil
}

// ── IOSDevice (real device) ───────────────────────────────────────────────────

var errIOSWebViewNotSupported = fmt.Errorf("not yet supported on real iOS devices")

// findAppPID returns the PID of the running process for the given bundle ID,
// using the same instruments + installationproxy pattern as TerminateApp.
func (d *IOSDevice) findAppPID(bundleID string) (int, error) {
	if err := d.startTunnel(); err != nil {
		return 0, fmt.Errorf("start tunnel: %w", err)
	}
	device, err := d.getEnhancedDevice()
	if err != nil {
		return 0, fmt.Errorf("get device: %w", err)
	}

	svc, err := installationproxy.New(device)
	if err != nil {
		return 0, fmt.Errorf("installationproxy: %w", err)
	}
	defer svc.Close()

	apps, err := svc.BrowseAllApps()
	if err != nil {
		return 0, fmt.Errorf("browse apps: %w", err)
	}
	var execName string
	for _, app := range apps {
		if app.CFBundleIdentifier() == bundleID {
			execName = app.CFBundleExecutable()
			break
		}
	}
	if execName == "" {
		return 0, fmt.Errorf("app %s not installed", bundleID)
	}

	infoSvc, err := instruments.NewDeviceInfoService(device)
	if err != nil {
		return 0, fmt.Errorf("device info service: %w", err)
	}
	defer infoSvc.Close()

	processes, err := infoSvc.ProcessList()
	if err != nil {
		return 0, fmt.Errorf("process list: %w", err)
	}
	for _, p := range processes {
		if p.Name == execName {
			return int(p.Pid), nil
		}
	}
	return 0, fmt.Errorf("process %q (%s) not found — is the app running?", execName, bundleID)
}

// writeIOSDeviceAgentDylib writes the embedded device dylib to a temp file.
func writeIOSDeviceAgentDylib() (string, error) {
	f, err := os.CreateTemp("", "mobilecli-agent-dev-*.dylib")
	if err != nil {
		return "", fmt.Errorf("create temp dylib: %w", err)
	}
	if _, err := f.Write(agents.IOSAgentDevDylib); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write dylib: %w", err)
	}
	f.Close()
	return f.Name(), nil
}

var tmpDirFromLLDB = regexp.MustCompile(`"(/private/var/[^"]+/tmp/)"`)

// injectIOSDeviceAgent injects the dylib into the given process on the device
// using two lldb sessions: one to discover NSTemporaryDirectory, one to push
// the file and dlopen it. Returns the device-side TCP port.
func injectIOSDeviceAgent(udid string, pid int, localDylibPath string) (int, error) {
	connect := fmt.Sprintf("platform connect connect://%s", udid)

	// pass 1: discover the app's temp directory
	out, _ := exec.Command("lldb",
		"-o", "platform select remote-ios",
		"-o", connect,
		"-o", fmt.Sprintf("process attach --pid %d", pid),
		"-o", `expr (const char*)[(NSString*)NSTemporaryDirectory() UTF8String]`,
		"-o", "detach",
		"-o", "quit",
	).CombinedOutput()

	m := tmpDirFromLLDB.FindStringSubmatch(string(out))
	if m == nil {
		return 0, fmt.Errorf("could not read NSTemporaryDirectory from process %d\nlldb output:\n%s", pid, out)
	}
	remoteDylibPath := m[1] + "mobilecli-agent.dylib"

	// pass 2: push dylib, dlopen, read port
	out, _ = exec.Command("lldb",
		"-o", "platform select remote-ios",
		"-o", connect,
		"-o", fmt.Sprintf("platform put-file %s %s", localDylibPath, remoteDylibPath),
		"-o", fmt.Sprintf("process attach --pid %d", pid),
		"-o", fmt.Sprintf("expr (void*)dlopen(%q, 2)", remoteDylibPath),
		"-o", "expr (int)mobilecli_get_port()",
		"-o", "detach",
		"-o", "quit",
	).CombinedOutput()

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "mobilecli_get_port") {
			continue
		}
		if m := portFromLLDB.FindStringSubmatch(line); m != nil {
			port, err := strconv.Atoi(m[1])
			if err == nil && port > 0 {
				return port, nil
			}
		}
	}
	return 0, fmt.Errorf("could not parse port from lldb output:\n%s", out)
}

// findFreeLocalPort returns the first available local TCP port in the given range.
func findFreeLocalPort(start, end int) (int, error) {
	for p := start; p <= end; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			ln.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in range %d-%d", start, end)
}

func (d *IOSDevice) ensureIOSDeviceAgentReady() (int, error) {
	// fast path: agent already forwarded from a previous call
	for port := 27042; port <= 27051; port++ {
		if isAgentReady(port) {
			return port, nil
		}
	}

	foreground, err := d.GetForegroundApp()
	if err != nil {
		return 0, fmt.Errorf("could not determine foreground app: %w", err)
	}

	pid, err := d.findAppPID(foreground.PackageName)
	if err != nil {
		return 0, err
	}

	localDylibPath, err := writeIOSDeviceAgentDylib()
	if err != nil {
		return 0, err
	}
	defer os.Remove(localDylibPath)

	devicePort, err := injectIOSDeviceAgent(d.Udid, pid, localDylibPath)
	if err != nil {
		return 0, fmt.Errorf("inject agent: %w", err)
	}

	localPort, err := findFreeLocalPort(27042, 27051)
	if err != nil {
		return 0, err
	}

	pf := iosutil.NewPortForwarder(d.Udid)
	if err := pf.Forward(localPort, devicePort); err != nil {
		return 0, fmt.Errorf("port forward %d->%d: %w", localPort, devicePort, err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for !isAgentReady(localPort) {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("iOS device agent did not respond on port %d within 3s", localPort)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return localPort, nil
}

func (d *IOSDevice) ListWebViews() ([]WebViewInfo, error) {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return nil, err
	}

	result, err := agentRequest(port, "device.webview.list", nil)
	if err != nil {
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

func (d *IOSDevice) WebViewGoto(webviewID, url string) error                           { return errIOSWebViewNotSupported }
func (d *IOSDevice) WebViewReload(webviewID string) error                              { return errIOSWebViewNotSupported }
func (d *IOSDevice) WebViewGoBack(webviewID string) error                              { return errIOSWebViewNotSupported }
func (d *IOSDevice) WebViewGoForward(webviewID string) error                           { return errIOSWebViewNotSupported }
func (d *IOSDevice) WebViewContent(webviewID string) (string, error)                   { return "", errIOSWebViewNotSupported }
func (d *IOSDevice) WebViewEvaluate(webviewID, expression string, args []any) (any, error) { return nil, errIOSWebViewNotSupported }
func (d *IOSDevice) WebViewWaitForLoadState(webviewID, state string, timeoutMs int) error { return errIOSWebViewNotSupported }
