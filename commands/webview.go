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
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	androidDevice, ok := device.(*devices.AndroidDevice)
	if !ok {
		return NewErrorResponse(fmt.Errorf("webview list is only supported on Android (device %s is %s)", device.ID(), device.Platform()))
	}

	foreground, err := androidDevice.GetForegroundApp()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("could not determine foreground app: %w", err))
	}

	webviews, err := androidDevice.ListWebViews(foreground.PackageName)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("webview list failed: %w", err))
	}

	return NewSuccessResponse(webviews)
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
