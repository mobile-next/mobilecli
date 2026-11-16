package devicekit

import (
	"slices"
	"strings"
	"testing"
)

func TestUnknownButtonErrorListsEverySupportedName(t *testing.T) {
	_, err := translateButton("BANANA")
	if err == nil {
		t.Fatal("translateButton(BANANA) succeeded, want an error")
	}

	for _, name := range ButtonNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestPowerAndLockAreTheSameButton(t *testing.T) {
	power, _ := translateButton("POWER")
	lock, _ := translateButton("LOCK")

	if power != "lock" || lock != "lock" {
		t.Errorf("POWER = %q, LOCK = %q, want both lock", power, lock)
	}
}

func TestButtonNamesIncludeEnter(t *testing.T) {
	if !slices.Contains(ButtonNames(), "ENTER") {
		t.Errorf("ButtonNames() = %v, want ENTER in it", ButtonNames())
	}
}
