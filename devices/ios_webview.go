package devices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mobile-next/mobilecli/agents"
	"github.com/mobile-next/mobilecli/utils"
)

// agentPortCache maps device UDID → local TCP port of its injected agent.
var (
	agentPortCache   = map[string]int{}
	agentPortCacheMu sync.Mutex
)

func cachedAgentPort(udid string) (int, bool) {
	agentPortCacheMu.Lock()
	defer agentPortCacheMu.Unlock()
	port, ok := agentPortCache[udid]
	return port, ok
}

func setCachedAgentPort(udid string, port int) {
	agentPortCacheMu.Lock()
	defer agentPortCacheMu.Unlock()
	agentPortCache[udid] = port
}

// findSimulatorPIDForBundle searches the Mac process list for the running
// process of a specific bundle ID inside the given simulator.
// Returns the PID and the .app bundle path.
func findSimulatorPIDForBundle(udid, bundleID string) (pid int, appBundlePath string, err error) {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		return 0, "", fmt.Errorf("ps aux: %w", err)
	}

	pattern := fmt.Sprintf(`CoreSimulator/Devices/%s/data/Containers/Bundle`, udid)

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, pattern) {
			continue
		}
		if strings.Contains(line, "UITests-Runner") || strings.Contains(line, ".xctrunner") {
			continue
		}
		m := psLineRegex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		p, parseErr := strconv.Atoi(m[1])
		if parseErr != nil {
			continue
		}
		bundle := appBundleFromExecPath(m[2])
		if bundle == "" {
			continue
		}
		bid, plistErr := bundleIDFromInfoPlist(filepath.Join(bundle, "Info.plist"))
		if plistErr != nil || bid != bundleID {
			continue
		}
		return p, bundle, nil
	}

	return 0, "", fmt.Errorf("no running process found for %s in simulator %s", bundleID, udid)
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

// psLineRegex matches a ps aux line and captures PID (group 1) and the full
// command string including spaces (group 2).
// Columns: USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND...
var psLineRegex = regexp.MustCompile(`^\S+\s+(\d+)(?:\s+\S+){8}\s+(.+)$`)

const lldbTimeout = 30 * time.Second

// injectIOSAgent attaches lldb to the given PID, loads the dylib, reads the
// bound port via mobilecli_get_port(), then detaches. Returns the port.
func injectIOSAgent(pid int, dylibPath string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), lldbTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lldb",
		"-p", strconv.Itoa(pid),
		"-o", fmt.Sprintf("expr (void*)dlopen(%q, 2)", dylibPath),
		"-o", "expr (int)mobilecli_get_port()",
		"-o", "detach",
		"-o", "quit",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return 0, fmt.Errorf("lldb timed out after %s", lldbTimeout)
		}
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
	// fast path: reuse the port we injected into this simulator previously
	if port, ok := cachedAgentPort(s.UDID); ok && isAgentReady(port) {
		return port, nil
	}

	if s.wdaClient == nil {
		if err := s.StartAgent(StartAgentConfig{}); err != nil {
			return 0, fmt.Errorf("webview commands require DeviceKit to be running — %w", err)
		}
	}
	foreground, err := s.GetForegroundApp()
	if err != nil {
		return 0, fmt.Errorf("could not determine foreground app: %w", err)
	}

	pid, appBundlePath, err := findSimulatorPIDForBundle(s.UDID, foreground.PackageName)
	if err != nil {
		return 0, fmt.Errorf("cannot attach to %s: app does not have get-task-allow entitlement — use a debug build", foreground.PackageName)
	}

	utils.Verbose("attaching to %s (pid %d, bundle %s)", appBundlePath, pid, foreground.PackageName)

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
	setCachedAgentPort(s.UDID, port)
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
