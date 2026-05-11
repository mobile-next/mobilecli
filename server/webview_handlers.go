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

type WebViewAttachParams struct {
	DeviceID  string `json:"deviceId"`
	WebViewID string `json:"id"`
}

type WebViewSessionParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
}

type WebViewGotoParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	URL       string `json:"url"`
	WaitUntil string `json:"waitUntil,omitempty"`
}

type WebViewReloadParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	WaitUntil string `json:"waitUntil,omitempty"`
}

type WebViewScreenshotParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	Format    string `json:"format,omitempty"`
	Quality   int    `json:"quality,omitempty"`
}

type WebViewEvaluateParams struct {
	DeviceID   string `json:"deviceId"`
	SessionID  string `json:"sessionId"`
	Expression string `json:"expression"`
	Args       []any  `json:"args,omitempty"`
}

type WebViewWaitForLoadStateParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	State     string `json:"state,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
}

type WebViewWaitForURLParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	URL       string `json:"url"`
	IsRegex   bool   `json:"isRegex,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
}

type WebViewQueryParams struct {
	DeviceID  string         `json:"deviceId"`
	SessionID string         `json:"sessionId"`
	Root      string         `json:"root,omitempty"`
	Strategy  string         `json:"strategy"`
	Value     string         `json:"value"`
	Options   map[string]any `json:"options,omitempty"`
	All       bool           `json:"all,omitempty"`
}

type WebViewElementNodeParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	NodeID    string `json:"nodeId"`
}

type WebViewElementFillParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	NodeID    string `json:"nodeId"`
	Value     string `json:"value"`
}

type WebViewElementTypeParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	NodeID    string `json:"nodeId"`
	Text      string `json:"text"`
}

type WebViewElementPressParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	NodeID    string `json:"nodeId"`
	Key       string `json:"key"`
}

type WebViewElementCheckParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	NodeID    string `json:"nodeId"`
	Checked   bool   `json:"checked"`
}

type WebViewElementSelectOptionParams struct {
	DeviceID  string   `json:"deviceId"`
	SessionID string   `json:"sessionId"`
	NodeID    string   `json:"nodeId"`
	Values    []string `json:"values"`
}

type WebViewElementAttributeParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	NodeID    string `json:"nodeId"`
	Name      string `json:"name"`
}

type WebViewElementWaitForParams struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	NodeID    string `json:"nodeId"`
	State     string `json:"state"`
	Timeout   int    `json:"timeout,omitempty"`
}

// ─── Shared helpers ───────────────────────────────────────────

// unmarshal parses a JSON-RPC params payload into the given type.
func unmarshal[T any](params json.RawMessage) (T, error) {
	var p T
	if err := json.Unmarshal(params, &p); err != nil {
		return p, fmt.Errorf("invalid parameters: %w", err)
	}
	return p, nil
}

// resultOf unwraps a CommandResponse, returning its data or an error.
func resultOf(resp *commands.CommandResponse) (any, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Data, nil
}

// voidOf unwraps a CommandResponse for operations that return no data.
func voidOf(resp *commands.CommandResponse) (any, error) {
	if resp.Status == "error" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return okResponse, nil
}

func requireSessionParams(deviceID, sessionID string) error {
	if deviceID == "" {
		return fmt.Errorf("deviceId is required")
	}
	if sessionID == "" {
		return fmt.Errorf("sessionId is required")
	}
	return nil
}

func requireElementParams(deviceID, sessionID, nodeID string) error {
	if err := requireSessionParams(deviceID, sessionID); err != nil {
		return err
	}
	if nodeID == "" {
		return fmt.Errorf("nodeId is required")
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

func handleWebViewAttach(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewAttachParams](params)
	if err != nil {
		return nil, err
	}
	if p.DeviceID == "" {
		return nil, fmt.Errorf("deviceId is required")
	}
	if p.WebViewID == "" {
		return nil, fmt.Errorf("id is required")
	}
	return resultOf(commands.WebViewAttachCommand(commands.WebViewAttachRequest{
		DeviceID:  p.DeviceID,
		WebViewID: p.WebViewID,
	}))
}

func handleWebViewDetach(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewSessionParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewDetachCommand(commands.WebViewSessionRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
	}))
}

func handleWebViewURL(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewSessionParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return resultOf(commands.WebViewURLCommand(commands.WebViewSessionRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
	}))
}

func handleWebViewTitle(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewSessionParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return resultOf(commands.WebViewTitleCommand(commands.WebViewSessionRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
	}))
}

func handleWebViewGoto(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewGotoParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	if p.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	return voidOf(commands.WebViewGotoCommand(commands.WebViewGotoRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		URL:       p.URL,
		WaitUntil: p.WaitUntil,
	}))
}

func handleWebViewReload(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewReloadParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewReloadCommand(commands.WebViewReloadRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		WaitUntil: p.WaitUntil,
	}))
}

func handleWebViewGoBack(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewSessionParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewGoBackCommand(commands.WebViewSessionRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
	}))
}

func handleWebViewGoForward(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewSessionParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewGoForwardCommand(commands.WebViewSessionRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
	}))
}

func handleWebViewContent(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewSessionParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return resultOf(commands.WebViewContentCommand(commands.WebViewSessionRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
	}))
}

func handleWebViewScreenshot(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewScreenshotParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return resultOf(commands.WebViewScreenshotCommand(commands.WebViewScreenshotRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		Format:    p.Format,
		Quality:   p.Quality,
	}))
}

func handleWebViewEvaluate(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewEvaluateParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	if p.Expression == "" {
		return nil, fmt.Errorf("expression is required")
	}
	return resultOf(commands.WebViewEvaluateCommand(commands.WebViewEvaluateRequest{
		DeviceID:   p.DeviceID,
		SessionID:  p.SessionID,
		Expression: p.Expression,
		Args:       p.Args,
	}))
}

func handleWebViewWaitForLoadState(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewWaitForLoadStateParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewWaitForLoadStateCommand(commands.WebViewWaitForLoadStateRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		State:     p.State,
		Timeout:   p.Timeout,
	}))
}

func handleWebViewWaitForURL(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewWaitForURLParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	if p.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	return voidOf(commands.WebViewWaitForURLCommand(commands.WebViewWaitForURLRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		URL:       p.URL,
		IsRegex:   p.IsRegex,
		Timeout:   p.Timeout,
	}))
}

func handleWebViewQuery(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewQueryParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireSessionParams(p.DeviceID, p.SessionID); err != nil {
		return nil, err
	}
	if p.Strategy == "" {
		return nil, fmt.Errorf("strategy is required")
	}
	if p.Value == "" {
		return nil, fmt.Errorf("value is required")
	}
	return resultOf(commands.WebViewQueryCommand(commands.WebViewQueryRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		Root:      p.Root,
		Strategy:  p.Strategy,
		Value:     p.Value,
		Options:   p.Options,
		All:       p.All,
	}))
}

func handleWebViewElementClick(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementNodeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewElementClickCommand(commands.WebViewElementNodeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
	}))
}

func handleWebViewElementFill(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementFillParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewElementFillCommand(commands.WebViewElementFillRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		Value:     p.Value,
	}))
}

func handleWebViewElementType(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementTypeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewElementTypeCommand(commands.WebViewElementTypeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		Text:      p.Text,
	}))
}

func handleWebViewElementPress(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementPressParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	if p.Key == "" {
		return nil, fmt.Errorf("key is required")
	}
	return voidOf(commands.WebViewElementPressCommand(commands.WebViewElementPressRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		Key:       p.Key,
	}))
}

func handleWebViewElementHover(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementNodeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewElementHoverCommand(commands.WebViewElementNodeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
	}))
}

func handleWebViewElementFocus(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementNodeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewElementFocusCommand(commands.WebViewElementNodeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
	}))
}

func handleWebViewElementScrollIntoView(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementNodeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewElementScrollIntoViewCommand(commands.WebViewElementNodeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
	}))
}

func handleWebViewElementCheck(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementCheckParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return voidOf(commands.WebViewElementCheckCommand(commands.WebViewElementCheckRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		Checked:   p.Checked,
	}))
}

func handleWebViewElementSelectOption(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementSelectOptionParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	if len(p.Values) == 0 {
		return nil, fmt.Errorf("values is required")
	}
	return voidOf(commands.WebViewElementSelectOptionCommand(commands.WebViewElementSelectOptionRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		Values:    p.Values,
	}))
}

func handleWebViewElementInspect(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementNodeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	return resultOf(commands.WebViewElementInspectCommand(commands.WebViewElementNodeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
	}))
}

func handleWebViewElementGetAttribute(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementAttributeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return resultOf(commands.WebViewElementGetAttributeCommand(commands.WebViewElementAttributeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		Name:      p.Name,
	}))
}

func handleWebViewElementGetProperty(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementAttributeParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return resultOf(commands.WebViewElementGetPropertyCommand(commands.WebViewElementAttributeRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		Name:      p.Name,
	}))
}

func handleWebViewElementWaitFor(params json.RawMessage) (any, error) {
	p, err := unmarshal[WebViewElementWaitForParams](params)
	if err != nil {
		return nil, err
	}
	if err := requireElementParams(p.DeviceID, p.SessionID, p.NodeID); err != nil {
		return nil, err
	}
	if p.State == "" {
		return nil, fmt.Errorf("state is required")
	}
	return voidOf(commands.WebViewElementWaitForCommand(commands.WebViewElementWaitForRequest{
		DeviceID:  p.DeviceID,
		SessionID: p.SessionID,
		NodeID:    p.NodeID,
		State:     p.State,
		Timeout:   p.Timeout,
	}))
}
