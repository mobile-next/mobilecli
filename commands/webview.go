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

// ─── Shared helper ────────────────────────────────────────────

// androidDeviceForWebView resolves the device and foreground package name,
// returning an error if the device is not Android.
func androidDeviceForWebView(deviceID string) (*devices.AndroidDevice, string, error) {
	device, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return nil, "", fmt.Errorf("error finding device: %w", err)
	}
	androidDevice, ok := device.(*devices.AndroidDevice)
	if !ok {
		return nil, "", fmt.Errorf("webview commands are only supported on Android (device %s is %s)", device.ID(), device.Platform())
	}
	foreground, err := androidDevice.GetForegroundApp()
	if err != nil {
		return nil, "", fmt.Errorf("could not determine foreground app: %w", err)
	}
	return androidDevice, foreground.PackageName, nil
}

// ─── Commands ─────────────────────────────────────────────────

func WebViewListCommand(req WebViewListRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	switch d := device.(type) {
	case *devices.AndroidDevice:
		foreground, err := d.GetForegroundApp()
		if err != nil {
			return NewErrorResponse(fmt.Errorf("could not determine foreground app: %w", err))
		}
		webviews, err := d.ListWebViews(foreground.PackageName)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("webview list failed: %w", err))
		}
		return NewSuccessResponse(webviews)

	case *devices.SimulatorDevice:
		webviews, err := d.ListWebViews()
		if err != nil {
			return NewErrorResponse(fmt.Errorf("webview list failed: %w", err))
		}
		return NewSuccessResponse(webviews)

	default:
		return NewErrorResponse(fmt.Errorf("webview list is not supported on %s (%s)", device.ID(), device.Platform()))
	}
}

func WebViewGotoCommand(req WebViewGotoRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}
	switch d := device.(type) {
	case *devices.AndroidDevice:
		foreground, err := d.GetForegroundApp()
		if err != nil {
			return NewErrorResponse(fmt.Errorf("could not determine foreground app: %w", err))
		}
		if err := d.WebViewGoto(foreground.PackageName, req.WebViewID, req.URL); err != nil {
			return NewErrorResponse(fmt.Errorf("webview goto failed: %w", err))
		}
	case *devices.SimulatorDevice:
		if err := d.WebViewGoto(req.WebViewID, req.URL); err != nil {
			return NewErrorResponse(fmt.Errorf("webview goto failed: %w", err))
		}
	default:
		return NewErrorResponse(fmt.Errorf("webview goto is not supported on %s (%s)", device.ID(), device.Platform()))
	}
	return NewSuccessResponse(OK)
}

func WebViewReloadCommand(req WebViewReloadRequest) *CommandResponse {
	androidDevice, pkg, err := androidDeviceForWebView(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := androidDevice.WebViewReload(pkg, req.WebViewID); err != nil {
		return NewErrorResponse(fmt.Errorf("webview reload failed: %w", err))
	}
	return NewSuccessResponse(OK)
}

func WebViewGoBackCommand(req WebViewRequest) *CommandResponse {
	androidDevice, pkg, err := androidDeviceForWebView(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := androidDevice.WebViewGoBack(pkg, req.WebViewID); err != nil {
		return NewErrorResponse(fmt.Errorf("webview back failed: %w", err))
	}
	return NewSuccessResponse(OK)
}

func WebViewGoForwardCommand(req WebViewRequest) *CommandResponse {
	androidDevice, pkg, err := androidDeviceForWebView(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := androidDevice.WebViewGoForward(pkg, req.WebViewID); err != nil {
		return NewErrorResponse(fmt.Errorf("webview forward failed: %w", err))
	}
	return NewSuccessResponse(OK)
}

func WebViewContentCommand(req WebViewRequest) *CommandResponse {
	androidDevice, pkg, err := androidDeviceForWebView(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	content, err := androidDevice.WebViewContent(pkg, req.WebViewID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("webview content failed: %w", err))
	}
	return NewSuccessResponse(content)
}

func WebViewEvaluateCommand(req WebViewEvaluateRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}
	switch d := device.(type) {
	case *devices.AndroidDevice:
		foreground, err := d.GetForegroundApp()
		if err != nil {
			return NewErrorResponse(fmt.Errorf("could not determine foreground app: %w", err))
		}
		result, err := d.WebViewEvaluate(foreground.PackageName, req.WebViewID, req.Expression, req.Args)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("webview evaluate failed: %w", err))
		}
		return NewSuccessResponse(result)
	case *devices.SimulatorDevice:
		result, err := d.WebViewEvaluate(req.WebViewID, req.Expression, req.Args)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("webview evaluate failed: %w", err))
		}
		return NewSuccessResponse(result)
	default:
		return NewErrorResponse(fmt.Errorf("webview evaluate is not supported on %s (%s)", device.ID(), device.Platform()))
	}
}

func WebViewWaitForLoadStateCommand(req WebViewWaitForLoadStateRequest) *CommandResponse {
	androidDevice, pkg, err := androidDeviceForWebView(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	if err := androidDevice.WebViewWaitForLoadState(pkg, req.WebViewID, req.State, req.Timeout); err != nil {
		return NewErrorResponse(fmt.Errorf("webview wait failed: %w", err))
	}
	return NewSuccessResponse(OK)
}
