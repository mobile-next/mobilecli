package commands

import (
	"fmt"
	"time"

	"github.com/mobile-next/mobilecli/devices"
)

const (
	keyboardCloseTimeout = 5 * time.Second
	keyboardPollInterval = 100 * time.Millisecond
)

type KeyboardRequest struct {
	DeviceID string `json:"deviceId"`
}

type KeyboardHideResult struct {
	Dismissed bool `json:"dismissed"`
}

type KeyboardStatusResult struct {
	Visible bool `json:"visible"`
}

func keyboardControllableDevice(deviceID string) (devices.KeyboardControllable, error) {
	device, err := FindDeviceWithAgent(deviceID)
	if err != nil {
		return nil, err
	}
	kb, ok := device.(devices.KeyboardControllable)
	if !ok {
		return nil, fmt.Errorf("keyboard commands are not supported on %s (%s)", device.ID(), device.Platform())
	}
	return kb, nil
}

// hideKeyboardAndWaitForClose returns false when no keyboard was showing, and an
// error when the keyboard cannot be seen to close before the timeout has passed.
// A failed status check is retried, because the hide itself has already worked.
func hideKeyboardAndWaitForClose(kb devices.KeyboardControllable, timeout time.Duration) (bool, error) {
	dismissed, err := kb.HideKeyboard()
	if err != nil || !dismissed {
		return false, err
	}

	deadline := time.Now().Add(timeout)
	for {
		visible, err := kb.IsKeyboardVisible()
		if err == nil && !visible {
			return true, nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return false, fmt.Errorf("could not confirm the keyboard closed: %w", err)
			}
			return false, fmt.Errorf("keyboard is still visible after %s", timeout)
		}
		time.Sleep(keyboardPollInterval)
	}
}

func KeyboardHideCommand(req KeyboardRequest) *CommandResponse {
	kb, err := keyboardControllableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}
	dismissed, err := hideKeyboardAndWaitForClose(kb, keyboardCloseTimeout)
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
