package devices

import (
	"fmt"
	"strconv"
	"strings"
)

// keycodeBack dismisses the on-screen keyboard when it's shown, same as a
// physical back-button press.
const keycodeBack = "4"

// parseInputShown extracts the mInputShown value from `dumpsys input_method`
// output. dumpsys packs multiple "key=value" fields on a single line (e.g.
// "mRequestedShowExplicitly=false mShowForced=false"), so mInputShown isn't
// always alone at the start of its line — each line is split into
// whitespace-separated fields to find it.
func parseInputShown(dumpsysOutput string) (bool, error) {
	for _, line := range strings.Split(dumpsysOutput, "\n") {
		for _, field := range strings.Fields(line) {
			value, ok := strings.CutPrefix(field, "mInputShown=")
			if !ok {
				continue
			}
			visible, err := strconv.ParseBool(value)
			if err != nil {
				return false, fmt.Errorf("parse mInputShown value %q: %w", value, err)
			}
			return visible, nil
		}
	}
	return false, fmt.Errorf("could not determine keyboard visibility: mInputShown not found in dumpsys input_method output")
}

// IsKeyboardVisible reports whether the on-screen keyboard is currently shown.
// This is a system-level property read via dumpsys, so it works regardless of
// which app is foregrounded and whether it's debuggable.
func (d *AndroidDevice) IsKeyboardVisible() (bool, error) {
	out, err := d.runAdbCommand("shell", "dumpsys", "input_method")
	if err != nil {
		return false, err
	}
	return parseInputShown(string(out))
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
