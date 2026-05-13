package devices

import (
	"encoding/json"
	"fmt"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
)

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

func (d *IOSDevice) ListWebViews() ([]WebViewInfo, error) {
	port, err := d.ensureIOSDeviceAgentReady()
	if err != nil {
		return nil, err
	}
	result, err := agentRequest(port, "device.webview.list", nil)
	if err != nil {
		setCachedAgentPort(d.Udid, 0)
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
