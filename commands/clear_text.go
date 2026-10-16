package commands

import "fmt"

type ClearTextRequest struct {
	DeviceID string `json:"deviceId"`
}

// ClearTextCommand empties the text field that has focus on the specified device.
func ClearTextCommand(req ClearTextRequest) *CommandResponse {
	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if err := targetDevice.ClearText(); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to clear text on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Cleared the focused text field on device %s", targetDevice.ID()),
	})
}
