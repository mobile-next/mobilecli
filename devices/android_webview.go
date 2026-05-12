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

// installWebViewKit pushes devicekit.so and devicekit.dex to the app's data
// directory and returns the agent directory path.
func (d *AndroidDevice) installWebViewKit(pkg string) (string, error) {
	dataDir, err := d.getAppDataDir(pkg)
	if err != nil {
		return "", err
	}
	agentDir := dataDir + "/" + agentSubDir

	const tmpSO = "/data/local/tmp/mobilecli-devicekit.so"
	const tmpDEX = "/data/local/tmp/mobilecli-devicekit.dex"

	if err := d.pushTempFile(agents.AndroidDevicekitSO, tmpSO); err != nil {
		return "", fmt.Errorf("push .so: %w", err)
	}
	if err := d.copyToAppDir(pkg, tmpSO, agentDir+"/devicekit.so", "755"); err != nil {
		return "", fmt.Errorf("install .so: %w", err)
	}

	if err := d.pushTempFile(agents.AndroidDevicekitDEX, tmpDEX); err != nil {
		return "", fmt.Errorf("push .dex: %w", err)
	}
	// remove stale dex before copying (dex is immutable once loaded)
	d.runAdbCommand("shell", "run-as", pkg, "rm", "-f", agentDir+"/devicekit.dex")
	if err := d.copyToAppDir(pkg, tmpDEX, agentDir+"/devicekit.dex", "444"); err != nil {
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
		Result json.RawMessage    `json:"result"`
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

// ensureAgentReady installs the kit, sets up the port forward, and attaches
// the agent if it is not already responding. Returns the local TCP port.
func (d *AndroidDevice) ensureAgentReady(pkg string) (int, error) {
	agentDir, err := d.installWebViewKit(pkg)
	if err != nil {
		return 0, fmt.Errorf("install webview kit: %w", err)
	}

	port, err := d.forwardWebViewSocket(pkg)
	if err != nil {
		return 0, fmt.Errorf("forward socket: %w", err)
	}

	if !isAgentReady(port) {
		pid, err := d.getProcessPID(pkg)
		if err != nil {
			return 0, err
		}
		if err := d.attachJVMTIAgent(pid, agentDir+"/devicekit.so", agentDir+"/devicekit.dex"); err != nil {
			return 0, fmt.Errorf("attach agent: %w", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for !isAgentReady(port) {
			if time.Now().After(deadline) {
				return 0, fmt.Errorf("agent did not start within 5s on port %d", port)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	return port, nil
}

// ListWebViews returns all embedded WebViews found in the given app.
func (d *AndroidDevice) ListWebViews(pkg string) ([]WebViewInfo, error) {
	port, err := d.ensureAgentReady(pkg)
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

// WebViewWaitForLoadState blocks until the webview reaches the given load state
// ("load" or "domcontentloaded"). timeoutMs of 0 uses the agent's default (30s).
func (d *AndroidDevice) WebViewWaitForLoadState(pkg, webviewID, state string, timeoutMs int) error {
	port, err := d.ensureAgentReady(pkg)
	if err != nil {
		return err
	}

	const agentDefaultMs = 30_000
	waitMs := agentDefaultMs
	if timeoutMs > 0 {
		waitMs = timeoutMs
	}

	params := map[string]any{"id": webviewID}
	if state != "" {
		params["state"] = state
	}
	params["timeout"] = waitMs

	// HTTP timeout must exceed the agent's blocking wait; add 5s buffer.
	httpTimeout := time.Duration(waitMs)*time.Millisecond + 5*time.Second
	_, err = agentRequestWithTimeout(port, "device.webview.waitForLoadState", params, httpTimeout)
	return err
}

// WebViewReload reloads the page in the given webview.
func (d *AndroidDevice) WebViewReload(pkg, webviewID string) error {
	port, err := d.ensureAgentReady(pkg)
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.reload", map[string]any{"id": webviewID})
	return err
}

// WebViewGoBack navigates the given webview back in its history.
func (d *AndroidDevice) WebViewGoBack(pkg, webviewID string) error {
	port, err := d.ensureAgentReady(pkg)
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goBack", map[string]any{"id": webviewID})
	return err
}

// WebViewGoForward navigates the given webview forward in its history.
func (d *AndroidDevice) WebViewGoForward(pkg, webviewID string) error {
	port, err := d.ensureAgentReady(pkg)
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goForward", map[string]any{"id": webviewID})
	return err
}

// WebViewContent returns the full outer HTML of the page in the given webview.
func (d *AndroidDevice) WebViewContent(pkg, webviewID string) (string, error) {
	result, err := d.WebViewEvaluate(pkg, webviewID, "return document.documentElement.outerHTML", nil)
	if err != nil {
		return "", err
	}
	content, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T", result)
	}
	return content, nil
}

// WebViewGoto navigates the given webview to url.
func (d *AndroidDevice) WebViewGoto(pkg, webviewID, url string) error {
	port, err := d.ensureAgentReady(pkg)
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goto", map[string]any{
		"id":  webviewID,
		"url": url,
	})
	return err
}

// ensureReturnExpression prepends "return" to bare expressions so the agent's
// eval wrapper can capture their value. Expressions that already start with
// "return", contain a statement separator, or look like a block are left as-is.
func ensureReturnExpression(expression string) string {
	trimmed := strings.TrimSpace(expression)
	if strings.HasPrefix(trimmed, "return ") ||
		strings.Contains(trimmed, ";") ||
		strings.Contains(trimmed, "\n") ||
		strings.HasPrefix(trimmed, "{") {
		return expression
	}
	return "return (" + trimmed + ")"
}

// WebViewEvaluate runs expression inside the given webview and returns the result.
// The Java agent wraps the value as {"result": <value>}; this method unwraps it.
func (d *AndroidDevice) WebViewEvaluate(pkg, webviewID, expression string, args []any) (any, error) {
	port, err := d.ensureAgentReady(pkg)
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
