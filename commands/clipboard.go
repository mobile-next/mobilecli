package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
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

	text, err := targetDevice.GetClipboard()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to read clipboard on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(ClipboardResult{Text: text})
}

func ClipboardSetCommand(req ClipboardSetRequest) *CommandResponse {
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

	err = targetDevice.SetClipboard(req.Text)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to write clipboard on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Clipboard set on device %s", targetDevice.ID()),
	})
}
