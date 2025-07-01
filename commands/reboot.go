package commands

import (
	"fmt"
)

// RebootRequest represents the parameters for a reboot command
type RebootRequest struct {
	DeviceID string `json:"deviceId"`
}

// RebootCommand reboots the specified device
func RebootCommand(req RebootRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.Reboot()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to reboot device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Reboot command processed for device %s", targetDevice.ID()),
	})
}
