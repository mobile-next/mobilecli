package commands

import (
	"fmt"
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

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = targetDevice.OpenURL(req.URL)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to open URL on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Opened URL '%s' on device %s", req.URL, targetDevice.ID()),
	})
}
