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
	if req.DeviceID == "" {
		return NewErrorResponse(fmt.Errorf("device ID is required"))
	}

	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	targetDevice, err := FindDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
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
	if req.DeviceID == "" {
		return NewErrorResponse(fmt.Errorf("device ID is required"))
	}

	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	targetDevice, err := FindDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.TerminateApp(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to terminate app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Terminated app '%s' on device %s", req.BundleID, targetDevice.ID()),
	})
}