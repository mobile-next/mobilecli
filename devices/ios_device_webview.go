package devices

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

// WebView feature for real iOS devices. These methods are thin JSON-RPC calls
// over the injected agent (see ios_device_agent.go for how the agent is
// injected and reached). They implement the WebViewable interface.
//
// Safari cannot take the agent, so when it is the foreground app its tabs are
// driven through the web inspector service instead (see ios_webinspector.go).

// safariInForeground returns what drives the tabs of Safari, when Safari is the
// app in front. Not knowing the foreground app leaves it to the injected agent,
// which reports why it cannot be reached.
func (d *IOSDevice) safariInForeground() (safariWebViews, bool) {
	if err := d.StartAgent(StartAgentConfig{}); err != nil {
		utils.Verbose("could not start the device agent to look up the foreground app: %v", err)
		return safariWebViews{}, false
	}
	activeApp, err := d.deviceKitClient.GetActiveAppInfo()
	if err != nil {
		utils.Verbose("could not look up the foreground app: %v", err)
		return safariWebViews{}, false
	}
	if activeApp.BundleID != safariBundleID {
		return safariWebViews{}, false
	}
	inspector := webInspectorOf(d.Udid, func() (io.ReadWriteCloser, error) { return dialDeviceWebInspector(d.Udid) })
	return safariWebViews{inspector: inspector}, true
}

func (d *IOSDevice) ListWebViews() ([]WebViewInfo, error) {
	if safari, ok := d.safariInForeground(); ok {
		return safari.ListWebViews()
	}
	result, err := d.agentCall("device.webview.list", nil)
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

func (d *IOSDevice) WebViewGoto(webviewID, url string) error {
	if safari, ok := d.safariInForeground(); ok {
		return safari.WebViewGoto(webviewID, url)
	}
	_, err := d.agentCall("device.webview.goto", map[string]any{"id": webviewID, "url": url})
	return err
}

func (d *IOSDevice) WebViewReload(webviewID string) error {
	if safari, ok := d.safariInForeground(); ok {
		return safari.WebViewReload(webviewID)
	}
	_, err := d.agentCall("device.webview.reload", map[string]any{"id": webviewID})
	return err
}

func (d *IOSDevice) WebViewGoBack(webviewID string) error {
	if safari, ok := d.safariInForeground(); ok {
		return safari.WebViewGoBack(webviewID)
	}
	_, err := d.agentCall("device.webview.goBack", map[string]any{"id": webviewID})
	return err
}

func (d *IOSDevice) WebViewGoForward(webviewID string) error {
	if safari, ok := d.safariInForeground(); ok {
		return safari.WebViewGoForward(webviewID)
	}
	_, err := d.agentCall("device.webview.goForward", map[string]any{"id": webviewID})
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
	if safari, ok := d.safariInForeground(); ok {
		return safari.WebViewEvaluate(webviewID, expression, args)
	}
	params := map[string]any{
		"id":         webviewID,
		"expression": ensureReturnExpression(expression),
	}
	if len(args) > 0 {
		params["args"] = args
	}
	raw, err := d.agentCall("device.webview.evaluate", params)
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
	if safari, ok := d.safariInForeground(); ok {
		return safari.WebViewWaitForLoadState(webviewID, state, timeoutMs)
	}
	if timeoutMs <= 0 {
		timeoutMs = 30_000
	}
	_, err := d.agentCallWithTimeout("device.webview.waitForLoadState", map[string]any{
		"id":      webviewID,
		"state":   state,
		"timeout": timeoutMs,
	}, time.Duration(timeoutMs+5000)*time.Millisecond)
	return err
}
