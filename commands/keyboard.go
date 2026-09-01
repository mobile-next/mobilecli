package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

type KeyboardRequest struct {
	DeviceID string
}

type KeyboardHideResult struct {
	Dismissed bool `json:"dismissed"`
}

type KeyboardStatusResult struct {
	Visible bool `json:"visible"`
}

func keyboardControllableDevice(deviceID string) (devices.KeyboardControllable, error) {
	device, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %w", err)
	}
	kb, ok := device.(devices.KeyboardControllable)
	if !ok {
		return nil, fmt.Errorf("keyboard commands are not supported on %s (%s)", device.ID(), device.Platform())
	}
	return kb, nil
}

func KeyboardHideCommand(req KeyboardRequest) *CommandResponse {
	kb, err := keyboardControllableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	dismissed, err := kb.HideKeyboard()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("keyboard hide failed: %w", err))
	}
	return NewSuccessResponse(KeyboardHideResult{Dismissed: dismissed})
}

func KeyboardStatusCommand(req KeyboardRequest) *CommandResponse {
	kb, err := keyboardControllableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	visible, err := kb.IsKeyboardVisible()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("keyboard status failed: %w", err))
	}
	return NewSuccessResponse(KeyboardStatusResult{Visible: visible})
}
