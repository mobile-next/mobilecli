package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// DeviceInfoResponse represents the response for a device info command
type DeviceInfoResponse struct {
	Device *devices.FullDeviceInfo `json:"device"`
}

func InfoCommand(deviceID string) *CommandResponse {
	targetDevice, err := FindDeviceWithAgent(deviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	info, err := targetDevice.Info()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error getting device info: %v", err))
	}

	response := DeviceInfoResponse{
		Device: info,
	}

	return NewSuccessResponse(response)
}
