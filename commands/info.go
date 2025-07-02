package commands

import (
	"fmt"
)

func InfoCommand(deviceID string) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	info, err := targetDevice.Info()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error getting device info: %v", err))
	}

	response := map[string]interface{}{
		"device": info,
	}

	return NewSuccessResponse(response)
}
