package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

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
}

type WebViewReloadRequest struct {
	DeviceID  string
	WebViewID string
}

type WebViewEvaluateRequest struct {
	DeviceID   string
	WebViewID  string
	Expression string
	Args       []any
}

type WebViewQueryRequest struct {
	DeviceID  string
	WebViewID string
	Selector  string
}

type WebViewWaitForLoadStateRequest struct {
	DeviceID  string
	WebViewID string
	State     string
	Timeout   int
}

// ─── Shared helper ────────────────────────────────────────────

func webViewableDevice(deviceID string) (devices.WebViewable, error) {
	device, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %w", err)
	}
	wv, ok := device.(devices.WebViewable)
	if !ok {
		return nil, fmt.Errorf("webview commands are not supported on %s (%s)", device.ID(), device.Platform())
	}
	return wv, nil
}

// ─── Commands ─────────────────────────────────────────────────

func WebViewListCommand(req WebViewListRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	webviews, err := wv.ListWebViews()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("webview list failed: %w", err))
	}
	return NewSuccessResponse(webviews)
}

func WebViewGotoCommand(req WebViewGotoRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := wv.WebViewGoto(req.WebViewID, req.URL); err != nil {
		return NewErrorResponse(fmt.Errorf("webview goto failed: %w", err))
	}
	return NewSuccessResponse(OK)
}

func WebViewReloadCommand(req WebViewReloadRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := wv.WebViewReload(req.WebViewID); err != nil {
		return NewErrorResponse(fmt.Errorf("webview reload failed: %w", err))
	}
	return NewSuccessResponse(OK)
}

func WebViewGoBackCommand(req WebViewRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := wv.WebViewGoBack(req.WebViewID); err != nil {
		return NewErrorResponse(fmt.Errorf("webview back failed: %w", err))
	}
	return NewSuccessResponse(OK)
}

func WebViewGoForwardCommand(req WebViewRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := wv.WebViewGoForward(req.WebViewID); err != nil {
		return NewErrorResponse(fmt.Errorf("webview forward failed: %w", err))
	}
	return NewSuccessResponse(OK)
}

func WebViewContentCommand(req WebViewRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	content, err := wv.WebViewContent(req.WebViewID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("webview content failed: %w", err))
	}
	return NewSuccessResponse(content)
}

func WebViewQueryCommand(req WebViewQueryRequest) *CommandResponse {
	expression := fmt.Sprintf(
		`Array.from(document.querySelectorAll(%q)).map(el => ({`+
			`tag: el.tagName.toLowerCase(),`+
			`text: (el.textContent || "").trim().slice(0, 200),`+
			`id: el.id || null,`+
			`class: el.className || null,`+
			`value: el.value || null,`+
			`href: el.href || null`+
			`}))`,
		req.Selector,
	)
	return WebViewEvaluateCommand(WebViewEvaluateRequest{
		DeviceID:   req.DeviceID,
		WebViewID:  req.WebViewID,
		Expression: expression,
	})
}

func WebViewEvaluateCommand(req WebViewEvaluateRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	result, err := wv.WebViewEvaluate(req.WebViewID, req.Expression, req.Args)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("webview evaluate failed: %w", err))
	}
	return NewSuccessResponse(result)
}

func WebViewWaitForLoadStateCommand(req WebViewWaitForLoadStateRequest) *CommandResponse {
	wv, err := webViewableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := wv.WebViewWaitForLoadState(req.WebViewID, req.State, req.Timeout); err != nil {
		return NewErrorResponse(fmt.Errorf("webview wait failed: %w", err))
	}
	return NewSuccessResponse(OK)
}
