package devices

import (
	"fmt"
	"strings"
)

// keycodeBack dismisses the on-screen keyboard when it's shown, same as a
// physical back-button press.
const keycodeBack = "4"

// IsKeyboardVisible reports whether the on-screen keyboard is currently shown.
// This is a system-level property read via dumpsys, so it works regardless of
// which app is foregrounded and whether it's debuggable.
func (d *AndroidDevice) IsKeyboardVisible() (bool, error) {
	out, err := d.runAdbCommand("shell", "dumpsys", "input_method")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(line, "mInputShown="); ok {
			return value == "true", nil
		}
	}
	return false, fmt.Errorf("could not determine keyboard visibility: mInputShown not found in dumpsys input_method output")
}

func (d *AndroidDevice) HideKeyboard() (bool, error) {
	visible, err := d.IsKeyboardVisible()
	if err != nil {
		return false, err
	}
	if !visible {
		return false, nil
	}
	if _, err := d.runAdbCommand("shell", "input", "keyevent", keycodeBack); err != nil {
		return false, err
	}
	return true, nil
}
