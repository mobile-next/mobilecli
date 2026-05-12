package devices

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/agents"
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
