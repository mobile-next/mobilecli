package devices

import (
	"strings"
	"testing"
)

func TestUnknownAndroidButtonErrorListsEverySupportedName(t *testing.T) {
	_, err := androidKeycode("BANANA")
	if err == nil {
		t.Fatal("androidKeycode(BANANA) succeeded, want an error")
	}

	for _, name := range AndroidButtonNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestAndroidPowerAndLockAreTheSameButton(t *testing.T) {
	power, _ := androidKeycode("POWER")
	lock, _ := androidKeycode("LOCK")

	if power != "KEYCODE_POWER" || lock != "KEYCODE_POWER" {
		t.Errorf("POWER = %q, LOCK = %q, want both KEYCODE_POWER", power, lock)
	}
}
