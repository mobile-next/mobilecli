package commands

import "fmt"

type FillRequest struct {
	DeviceID string `json:"deviceId"`
	Text     string `json:"text"`
}

// FillCommand replaces the focused text field's text on the specified device; "" clears it.
func FillCommand(req FillRequest) *CommandResponse {
	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if err := targetDevice.Fill(req.Text); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to fill text on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Filled the focused text field on device %s", targetDevice.ID()),
	})
}
