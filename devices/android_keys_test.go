package devices

import "testing"

// The cross-platform "command" modifier means the primary shortcut key. On
// Android that is Control (Meta+A opens the Assistant instead of selecting
// text), so cmd+a must send Ctrl+A to match the documented select-all behavior.
func TestAndroidCommandModifierMapsToControl(t *testing.T) {
	if got := androidModifierKeycodes["command"]; got != "KEYCODE_CTRL_LEFT" {
		t.Errorf("command modifier = %q, want KEYCODE_CTRL_LEFT (Android primary shortcut is Control)", got)
	}
	if got := androidModifierKeycodes["control"]; got != "KEYCODE_CTRL_LEFT" {
		t.Errorf("control modifier = %q, want KEYCODE_CTRL_LEFT", got)
	}
	// Meta must not be the mapping for command, or cmd+a opens the Assistant.
	if androidModifierKeycodes["command"] == "KEYCODE_META_LEFT" {
		t.Error("command modifier must not map to KEYCODE_META_LEFT on Android")
	}
}
