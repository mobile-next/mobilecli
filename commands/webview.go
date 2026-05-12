package commands

import "fmt"

// ─── Request types ────────────────────────────────────────────

type WebViewListRequest struct {
	DeviceID string
}

// WebViewRequest is the base for all webview operations that target a specific webview.
type WebViewRequest struct {
	DeviceID  string
	WebViewID string
}

type WebViewGotoRequest struct {
	DeviceID  string
	WebViewID string
	URL       string
	WaitUntil string
}

type WebViewReloadRequest struct {
	DeviceID  string
	WebViewID string
	WaitUntil string
}

type WebViewEvaluateRequest struct {
	DeviceID   string
	WebViewID  string
	Expression string
	Args       []any
}

type WebViewWaitForLoadStateRequest struct {
	DeviceID  string
	WebViewID string
	State     string
	Timeout   int
}

// ─── Stubs ────────────────────────────────────────────────────

func WebViewListCommand(req WebViewListRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewGotoCommand(req WebViewGotoRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewReloadCommand(req WebViewReloadRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewGoBackCommand(req WebViewRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewGoForwardCommand(req WebViewRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewEvaluateCommand(req WebViewEvaluateRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewWaitForLoadStateCommand(req WebViewWaitForLoadStateRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}
