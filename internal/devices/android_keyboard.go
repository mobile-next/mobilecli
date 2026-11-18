package devices

import (
	"encoding/json"
	"fmt"
)

type keyboardStatusResult struct {
	Visible bool `json:"visible"`
}

type keyboardHideResult struct {
	Dismissed bool `json:"dismissed"`
}

// IsKeyboardVisible reports whether an input method window is on screen.
func (d *AndroidDevice) IsKeyboardVisible() (bool, error) {
	raw, err := d.serverRequest("device.io.keyboard.status", nil)
	if err != nil {
		return false, err
	}
	var result keyboardStatusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, fmt.Errorf("failed to parse keyboard status response: %w", err)
	}
	return result.Visible, nil
}

// HideKeyboard presses BACK if the keyboard is showing. It returns false when
// no keyboard was showing, so it never navigates back on its own.
func (d *AndroidDevice) HideKeyboard() (bool, error) {
	raw, err := d.serverRequest("device.io.keyboard.hide", nil)
	if err != nil {
		return false, err
	}
	var result keyboardHideResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, fmt.Errorf("failed to parse keyboard hide response: %w", err)
	}
	return result.Dismissed, nil
}
