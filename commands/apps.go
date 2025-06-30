package commands

import (
	"fmt"
)

// AppRequest represents the parameters for app-related commands
type AppRequest struct {
	DeviceID string `json:"deviceId"`
	BundleID string `json:"bundleId"`
}

// LaunchAppCommand launches an app on the specified device
func LaunchAppCommand(req AppRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.LaunchApp(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to launch app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Launched app '%s' on device %s", req.BundleID, targetDevice.ID()),
	})
}

// TerminateAppCommand terminates an app on the specified device
func TerminateAppCommand(req AppRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.TerminateApp(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to terminate app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Terminated app '%s' on device %s", req.BundleID, targetDevice.ID()),
	})
}

// ListAppsRequest represents the parameters for listing apps
type ListAppsRequest struct {
	DeviceID string `json:"deviceId"`
}

// ListAppsCommand lists installed apps on a device
func ListAppsCommand(req ListAppsRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	apps, err := targetDevice.ListApps()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to list apps on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(apps)
}
