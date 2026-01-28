package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// URLRequest represents the parameters for a URL opening command
type URLRequest struct {
	DeviceID string `json:"deviceId"`
	URL      string `json:"url"`
}

// URLCommand opens a URL on the specified device
func URLCommand(req URLRequest) *CommandResponse {
	if req.URL == "" {
		return NewErrorResponse(fmt.Errorf("URL is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.OpenURL(req.URL)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to open URL on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Opened URL '%s' on device %s", req.URL, targetDevice.ID()),
	})
}
