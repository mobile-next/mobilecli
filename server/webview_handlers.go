package server

import (
	"encoding/json"
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
)

// ─── Params structs ───────────────────────────────────────────

type WebViewListParams struct {
	DeviceID string `json:"deviceId"`
}

type WebViewParams struct {
	DeviceID  string `json:"deviceId"`
	WebViewID string `json:"id"`
}

type WebViewGotoParams struct {
	DeviceID  string `json:"deviceId"`
	WebViewID string `json:"id"`
	URL       string `json:"url"`
}

type WebViewReloadParams struct {
	DeviceID  string `json:"deviceId"`
	WebViewID string `json:"id"`
}

type WebViewEvaluateParams struct {
	DeviceID   string `json:"deviceId"`
	WebViewID  string `json:"id"`
	Expression string `json:"expression"`
	Args       []any  `json:"args,omitempty"`
}

type WebViewQueryParams struct {
	DeviceID  string `json:"deviceId"`
	WebViewID string `json:"id"`
	Selector  string `json:"selector"`
}

type WebViewWaitForLoadStateParams struct {
	DeviceID  string `json:"deviceId"`
	WebViewID string `json:"id"`
	State     string `json:"state,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
}

// ─── Shared helpers ───────────────────────────────────────────

func unmarshal[T any](params json.RawMessage) (T, error) {
	var p T
	if err := json.Unmarshal(params, &p); err != nil {
		return p, fmt.Errorf("invalid parameters: %w", err)
	}
	return p, nil
}

func resultOf(resp *commands.CommandResponse) (any, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Data, nil
}

func voidOf(resp *commands.CommandResponse) (any, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return okResponse, nil
}

func requireWebViewParams(deviceID, webViewID string) error {
	if deviceID == "" {
		return fmt.Errorf("deviceId is required")
	}
	if webViewID == "" {
		return fmt.Errorf("id is required")
	}
	return nil
}

// ─── Handlers ─────────────────────────────────────────────────

func handleWebViewList(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewListParams](params)
	if err != nil {
		return nil, err
	}
	if p.DeviceID == "" {
		return nil, fmt.Errorf("deviceId is required")
	}
	return resultOf(commands.WebViewListCommand(commands.WebViewListRequest{
		DeviceID: p.DeviceID,
	}))
}

func handleWebViewGoto(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewGotoParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	if p.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	return voidOf(commands.WebViewGotoCommand(commands.WebViewGotoRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
		URL:       p.URL,
	}))
}

func handleWebViewReload(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewReloadParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewReloadCommand(commands.WebViewReloadRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
	}))
}

func handleWebViewGoBack(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewGoBackCommand(commands.WebViewRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
	}))
}

func handleWebViewGoForward(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewGoForwardCommand(commands.WebViewRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
	}))
}

func handleWebViewContent(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	return resultOf(commands.WebViewContentCommand(commands.WebViewRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
	}))
}

// evaluateFixedExpression runs a constant JS expression against a webview,
// used by handlers that are convenience wrappers around evaluate (url, title).
func evaluateFixedExpression(params json.RawMessage, expression string) (any, error) {
	p, err := unmarshal[WebViewParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	return resultOf(commands.WebViewEvaluateCommand(commands.WebViewEvaluateRequest{
		DeviceID:   p.DeviceID,
		WebViewID:  p.WebViewID,
		Expression: expression,
	}))
}

func handleWebViewURL(params json.RawMessage) (any, error) {
	return evaluateFixedExpression(params, "return location.href")
}

func handleWebViewTitle(params json.RawMessage) (any, error) {
	return evaluateFixedExpression(params, "return document.title")
}

func handleWebViewQuery(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewQueryParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	if p.Selector == "" {
		return nil, fmt.Errorf("selector is required")
	}
	return resultOf(commands.WebViewQueryCommand(commands.WebViewQueryRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
		Selector:  p.Selector,
	}))
}

func handleWebViewEvaluate(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewEvaluateParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	if p.Expression == "" {
		return nil, fmt.Errorf("expression is required")
	}
	return resultOf(commands.WebViewEvaluateCommand(commands.WebViewEvaluateRequest{
		DeviceID:   p.DeviceID,
		WebViewID:  p.WebViewID,
		Expression: p.Expression,
		Args:       p.Args,
	}))
}

func handleWebViewWaitForLoadState(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewWaitForLoadStateParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireWebViewParams(p.DeviceID, p.WebViewID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewWaitForLoadStateCommand(commands.WebViewWaitForLoadStateRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
		State:     p.State,
		Timeout:   p.Timeout,
	}))
}
