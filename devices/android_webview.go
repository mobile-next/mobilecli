package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/agents"
	"github.com/mobile-next/mobilecli/utils"
)

// WebViewInfo describes an embedded WebView found inside a running app.
type WebViewInfo struct {
	ID          string         `json:"id"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	BundleID    string         `json:"bundleId"`
	ProcessName string         `json:"processName"`
	Bounds      map[string]any `json:"bounds,omitempty"`
	IsVisible   bool           `json:"isVisible"`
}

const agentSubDir = "mobilecli"

// pushTempFile writes data to a host temp file then pushes it to the device
// at remotePath using adb push.
func (d *AndroidDevice) pushTempFile(data []byte, remotePath string) error {
	tmp, err := os.CreateTemp("", "mobilecli-agent-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	out, err := d.runAdbCommand("push", tmp.Name(), remotePath)
	if err != nil {
		return fmt.Errorf("adb push to %s: %s: %w", remotePath, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// getAppDataDir returns the data directory of the given package by running
// pwd as the app user.
func (d *AndroidDevice) getAppDataDir(pkg string) (string, error) {
	out, err := d.runAdbCommand("shell", "run-as", pkg, "pwd")
	if err != nil {
		// fall back to the conventional path
		return "/data/data/" + pkg, nil
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "/data/data/" + pkg, nil
	}
	return dir, nil
}

// copyToAppDir copies a file from /data/local/tmp into the app's data directory
// using run-as, then sets the given chmod mode.
func (d *AndroidDevice) copyToAppDir(pkg, tmpPath, destPath, mode string) error {
	if out, err := d.runAdbCommand("shell", "run-as", pkg, "mkdir", "-p", destPath[:strings.LastIndex(destPath, "/")]); err != nil {
		return fmt.Errorf("mkdir in app dir: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := d.runAdbCommand("shell", "run-as", pkg, "cp", tmpPath, destPath); err != nil {
		return fmt.Errorf("cp to app dir: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := d.runAdbCommand("shell", "run-as", pkg, "chmod", mode, destPath); err != nil {
		return fmt.Errorf("chmod %s %s: %s: %w", mode, destPath, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// installWebViewKit pushes mobilecli.so and mobilecli.dex to the app's data
// directory and returns the agent directory path.
func (d *AndroidDevice) installWebViewKit(pkg string) (string, error) {
	dataDir, err := d.getAppDataDir(pkg)
	if err != nil {
		return "", err
	}
	agentDir := dataDir + "/" + agentSubDir

	const tmpSO = "/data/local/tmp/mobilecli.so"
	const tmpDEX = "/data/local/tmp/mobilecli.dex"

	if err := d.pushTempFile(agents.AndroidMobilecliSO, tmpSO); err != nil {
		return "", fmt.Errorf("push .so: %w", err)
	}
	if err := d.copyToAppDir(pkg, tmpSO, agentDir+"/mobilecli.so", "755"); err != nil {
		return "", fmt.Errorf("install .so: %w", err)
	}

	if err := d.pushTempFile(agents.AndroidMobilecliDEX, tmpDEX); err != nil {
		return "", fmt.Errorf("push .dex: %w", err)
	}
	// remove stale dex before copying (dex is immutable once loaded)
	d.runAdbCommand("shell", "run-as", pkg, "rm", "-f", agentDir+"/mobilecli.dex")
	if err := d.copyToAppDir(pkg, tmpDEX, agentDir+"/mobilecli.dex", "444"); err != nil {
		return "", fmt.Errorf("install .dex: %w", err)
	}

	return agentDir, nil
}

// forwardWebViewSocket creates an adb forward from a random local TCP port to
// the agent's local abstract socket and returns the assigned port.
func (d *AndroidDevice) forwardWebViewSocket(pkg string) (int, error) {
	out, err := d.runAdbCommand("forward", "tcp:0", "localabstract:mobilecli."+pkg)
	if err != nil {
		return 0, fmt.Errorf("adb forward: %s: %w", strings.TrimSpace(string(out)), err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("unexpected adb forward output %q: %w", strings.TrimSpace(string(out)), err)
	}
	return port, nil
}

// getProcessPID returns the PID of the running process for the given package.
func (d *AndroidDevice) getProcessPID(pkg string) (string, error) {
	out, err := d.runAdbCommand("shell", "pidof", "-s", pkg)
	if err != nil {
		return "", fmt.Errorf("pidof %s: %s: %w", pkg, strings.TrimSpace(string(out)), err)
	}
	pid := strings.TrimSpace(string(out))
	if pid == "" {
		return "", fmt.Errorf("no running process found for %s — is the app open?", pkg)
	}
	return pid, nil
}

// attachJVMTIAgent attaches the .so to the running process via am attach-agent,
// passing the dex path as the agent option (agent.so=<dex_path>).
func (d *AndroidDevice) attachJVMTIAgent(pid, soPath, dexPath string) error {
	agentArg := soPath + "=" + dexPath
	out, err := d.runAdbCommand("shell", "am", "attach-agent", pid, agentArg)
	if err != nil {
		return fmt.Errorf("am attach-agent: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

const defaultAgentTimeout = 10 * time.Second

// agentRequest sends a JSON-RPC 2.0 request to the agent over HTTP and returns
// the result field from the response.
func agentRequest(port int, method string, params map[string]any) (json.RawMessage, error) {
	return agentRequestWithTimeout(port, method, params, defaultAgentTimeout)
}

func agentRequestWithTimeout(port int, method string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  method,
	}
	if len(params) > 0 {
		body["params"] = params
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	start := time.Now()
	defer func() {
		utils.Verbose("agentRequest method=%s payloadBytes=%d elapsed=%s", method, len(payload), time.Since(start))
	}()

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(
		fmt.Sprintf("http://localhost:%d/", port),
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to agent on port %d: %w", port, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read agent response: %w", err)
	}

	var rpc struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &rpc); err != nil {
		return nil, fmt.Errorf("parse agent response: %w", err)
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("agent error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	return rpc.Result, nil
}

// isAgentReady checks whether the agent socket is already accepting connections.
func isAgentReady(port int) bool {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Post(fmt.Sprintf("http://localhost:%d/", port), "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// isAppDebuggable returns true when run-as can execute commands as the app user.
func (d *AndroidDevice) isAppDebuggable(pkg string) bool {
	_, err := d.runAdbCommand("shell", "run-as", pkg, "true")
	return err == nil
}

// findExistingForward checks adb's active forward table for an entry pointing
// to the agent's abstract socket and returns the host TCP port, or 0 if none.
func (d *AndroidDevice) findExistingForward(pkg string) int {
	out, err := d.runAdbCommand("forward", "--list")
	if err != nil {
		return 0
	}
	target := "localabstract:mobilecli." + pkg
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if !strings.Contains(line, target) {
			continue
		}
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "tcp:") {
				if port, err := strconv.Atoi(strings.TrimPrefix(field, "tcp:")); err == nil && port > 0 {
					return port
				}
			}
		}
	}
	return 0
}

// ensureAgentReady ensures the webview agent is running and reachable.
// It reuses an existing adb forward when possible, only creating a new one
// on the first call or after the forward has been removed.
func (d *AndroidDevice) ensureAgentReady(pkg string) (int, error) {
	// fast path: reuse an existing forward if the agent is still up
	if port := d.findExistingForward(pkg); port != 0 {
		if isAgentReady(port) {
			return port, nil
		}
		// forward exists but agent is gone (app restarted) — re-attach to the
		// new process; the existing forward still maps the same socket name
		agentDir, err := d.installWebViewKit(pkg)
		if err != nil {
			return 0, fmt.Errorf("install webview kit: %w", err)
		}
		if err := d.attachAgentAndWait(pkg, port, agentDir); err != nil {
			return 0, err
		}
		return port, nil
	}

	// no existing forward: full setup
	if !d.isAppDebuggable(pkg) {
		return 0, fmt.Errorf("webview injection requires a debug build: %s is not debuggable", pkg)
	}

	agentDir, err := d.installWebViewKit(pkg)
	if err != nil {
		return 0, fmt.Errorf("install webview kit: %w", err)
	}

	port, err := d.forwardWebViewSocket(pkg)
	if err != nil {
		return 0, fmt.Errorf("forward socket: %w", err)
	}

	if !isAgentReady(port) {
		if err := d.attachAgentAndWait(pkg, port, agentDir); err != nil {
			return 0, err
		}
	}

	return port, nil
}

// attachAgentAndWait gets the current PID, attaches the JVMTI agent, and waits
// up to 5 seconds for the agent to start accepting connections on port.
func (d *AndroidDevice) attachAgentAndWait(pkg string, port int, agentDir string) error {
	pid, err := d.getProcessPID(pkg)
	if err != nil {
		return err
	}
	if err := d.attachJVMTIAgent(pid, agentDir+"/mobilecli.so", agentDir+"/mobilecli.dex"); err != nil {
		return fmt.Errorf("attach agent: %w", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !isAgentReady(port) {
		if time.Now().After(deadline) {
			return fmt.Errorf("agent did not start within 5s on port %d", port)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// getWebViewPort resolves the foreground app and ensures the agent is ready,
// returning the local TCP port to use for RPC calls.
func (d *AndroidDevice) getWebViewPort() (int, error) {
	// Foreground detection can momentarily fail right after a launch or in-app
	// navigation — mCurrentFocus is briefly null during the window transition —
	// so retry for a short while instead of failing on the first miss.
	var foreground *ForegroundAppInfo
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		foreground, err = d.GetForegroundApp()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("could not determine foreground app: %w", err)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return d.ensureAgentReady(foreground.PackageName)
}

func (d *AndroidDevice) ListWebViews() ([]WebViewInfo, error) {
	port, err := d.getWebViewPort()
	if err != nil {
		return nil, err
	}
	result, err := agentRequest(port, "device.webview.list", nil)
	if err != nil {
		return nil, err
	}
	var webviews []WebViewInfo
	if err := json.Unmarshal(result, &webviews); err != nil {
		return nil, fmt.Errorf("parse webview list: %w", err)
	}
	return webviews, nil
}

func (d *AndroidDevice) WebViewGoto(webviewID, url string) error {
	port, err := d.getWebViewPort()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goto", map[string]any{"id": webviewID, "url": url})
	return err
}

func (d *AndroidDevice) WebViewReload(webviewID string) error {
	port, err := d.getWebViewPort()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.reload", map[string]any{"id": webviewID})
	return err
}

func (d *AndroidDevice) WebViewGoBack(webviewID string) error {
	port, err := d.getWebViewPort()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goBack", map[string]any{"id": webviewID})
	return err
}

func (d *AndroidDevice) WebViewGoForward(webviewID string) error {
	port, err := d.getWebViewPort()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goForward", map[string]any{"id": webviewID})
	return err
}

func (d *AndroidDevice) WebViewContent(webviewID string) (string, error) {
	result, err := d.WebViewEvaluate(webviewID, "return document.documentElement.outerHTML", nil)
	if err != nil {
		return "", err
	}
	content, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T", result)
	}
	return content, nil
}

// ensureReturnExpression turns a value expression into a statement body that
// returns that value, so the agent's eval wrapper can capture it. A bare
// expression — even an IIFE that internally uses ';', '{' or newlines — must be
// wrapped; only skip wrapping when the caller already supplied a top-level
// "return". A trailing ';' is stripped so the wrapped form stays valid.
func ensureReturnExpression(expression string) string {
	trimmed := strings.TrimSpace(expression)
	if strings.HasPrefix(trimmed, "return ") || strings.HasPrefix(trimmed, "return(") {
		return expression
	}
	trimmed = strings.TrimRight(trimmed, " \t\r\n;")
	return "return (" + trimmed + ")"
}

func (d *AndroidDevice) WebViewEvaluate(webviewID, expression string, args []any) (any, error) {
	port, err := d.getWebViewPort()
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
	// agent returns {"result": <value>} — unwrap one level
	var wrapper struct {
		Result any `json:"result"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse evaluate result: %w", err)
	}
	return wrapper.Result, nil
}

func (d *AndroidDevice) WebViewWaitForLoadState(webviewID, state string, timeoutMs int) error {
	port, err := d.getWebViewPort()
	if err != nil {
		return err
	}
	const agentDefaultMs = 30_000
	waitMs := agentDefaultMs
	if timeoutMs > 0 {
		waitMs = timeoutMs
	}
	params := map[string]any{"id": webviewID, "timeout": waitMs}
	if state != "" {
		params["state"] = state
	}
	httpTimeout := time.Duration(waitMs)*time.Millisecond + 5*time.Second
	_, err = agentRequestWithTimeout(port, "device.webview.waitForLoadState", params, httpTimeout)
	return err
}
