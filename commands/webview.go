package commands

import "fmt"

// ─── Request types ────────────────────────────────────────────

type WebViewListRequest struct {
	DeviceID string
}

type WebViewAttachRequest struct {
	DeviceID  string
	WebViewID string
}

// WebViewSessionRequest is the base for all session-scoped operations.
type WebViewSessionRequest struct {
	DeviceID  string
	SessionID string
}

type WebViewGotoRequest struct {
	DeviceID  string
	SessionID string
	URL       string
	WaitUntil string
}

type WebViewReloadRequest struct {
	DeviceID  string
	SessionID string
	WaitUntil string
}

type WebViewScreenshotRequest struct {
	DeviceID  string
	SessionID string
	Format    string
	Quality   int
}

type WebViewEvaluateRequest struct {
	DeviceID   string
	SessionID  string
	Expression string
	Args       []any
}

type WebViewWaitForLoadStateRequest struct {
	DeviceID  string
	SessionID string
	State     string
	Timeout   int
}

type WebViewWaitForURLRequest struct {
	DeviceID  string
	SessionID string
	URL       string
	IsRegex   bool
	Timeout   int
}

type WebViewQueryRequest struct {
	DeviceID  string
	SessionID string
	Root      string
	Strategy  string
	Value     string
	Options   map[string]any
	All       bool
}

type WebViewElementNodeRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
}

type WebViewElementFillRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
	Value     string
}

type WebViewElementTypeRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
	Text      string
}

type WebViewElementPressRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
	Key       string
}

type WebViewElementCheckRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
	Checked   bool
}

type WebViewElementSelectOptionRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
	Values    []string
}

type WebViewElementAttributeRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
	Name      string
}

type WebViewElementWaitForRequest struct {
	DeviceID  string
	SessionID string
	NodeID    string
	State     string
	Timeout   int
}

// ─── Stubs ────────────────────────────────────────────────────

func WebViewListCommand(req WebViewListRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewAttachCommand(req WebViewAttachRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewDetachCommand(req WebViewSessionRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewURLCommand(req WebViewSessionRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewTitleCommand(req WebViewSessionRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewGotoCommand(req WebViewGotoRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewReloadCommand(req WebViewReloadRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewGoBackCommand(req WebViewSessionRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewGoForwardCommand(req WebViewSessionRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewContentCommand(req WebViewSessionRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewScreenshotCommand(req WebViewScreenshotRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewEvaluateCommand(req WebViewEvaluateRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewWaitForLoadStateCommand(req WebViewWaitForLoadStateRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewWaitForURLCommand(req WebViewWaitForURLRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewQueryCommand(req WebViewQueryRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementClickCommand(req WebViewElementNodeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementFillCommand(req WebViewElementFillRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementTypeCommand(req WebViewElementTypeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementPressCommand(req WebViewElementPressRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementHoverCommand(req WebViewElementNodeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementFocusCommand(req WebViewElementNodeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementScrollIntoViewCommand(req WebViewElementNodeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementCheckCommand(req WebViewElementCheckRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementSelectOptionCommand(req WebViewElementSelectOptionRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementInspectCommand(req WebViewElementNodeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementGetAttributeCommand(req WebViewElementAttributeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementGetPropertyCommand(req WebViewElementAttributeRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}

func WebViewElementWaitForCommand(req WebViewElementWaitForRequest) *CommandResponse {
	return NewErrorResponse(fmt.Errorf("not implemented"))
}
