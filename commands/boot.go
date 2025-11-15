package commands

import (
	"fmt"
)

// BootRequest represents the parameters for a boot command
type BootRequest struct {
	DeviceID string `json:"deviceId"`
}

// BootCommand boots the specified simulator or emulator
func BootCommand(req BootRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.Boot()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to boot device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message":  fmt.Sprintf("Device %s booted successfully", targetDevice.ID()),
		"platform": targetDevice.Platform(),
		"type":     targetDevice.DeviceType(),
		"version":  targetDevice.Version(),
	})
}

// ShutdownRequest represents the parameters for a shutdown command
type ShutdownRequest struct {
	DeviceID string `json:"deviceId"`
}

// ShutdownCommand shuts down the specified simulator or emulator
func ShutdownCommand(req ShutdownRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.Shutdown()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to shutdown device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message":  fmt.Sprintf("Device %s shut down successfully", targetDevice.ID()),
		"platform": targetDevice.Platform(),
		"type":     targetDevice.DeviceType(),
		"version":  targetDevice.Version(),
	})
}
