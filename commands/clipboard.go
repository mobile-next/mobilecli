package commands

import (
	"fmt"
)

type ClipboardGetRequest struct {
	DeviceID string `json:"deviceId"`
}

type ClipboardSetRequest struct {
	DeviceID string `json:"deviceId"`
	Text     string `json:"text"`
}

type ClipboardResult struct {
	Text string `json:"text"`
}

func ClipboardGetCommand(req ClipboardGetRequest) *CommandResponse {
	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	text, err := targetDevice.GetClipboard()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to read clipboard on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(ClipboardResult{Text: text})
}

func ClipboardSetCommand(req ClipboardSetRequest) *CommandResponse {
	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = targetDevice.SetClipboard(req.Text)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to write clipboard on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Clipboard set on device %s", targetDevice.ID()),
	})
}
