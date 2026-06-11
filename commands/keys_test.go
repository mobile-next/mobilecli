package commands

import (
	"reflect"
	"testing"

	"github.com/mobile-next/mobilecli/devices"
)

func TestParseKeyCombo(t *testing.T) {
	tests := []struct {
		combo    string
		expected devices.KeyCombo
	}{
		{"cmd+a", devices.KeyCombo{Key: "a", Modifiers: []string{"command"}}},
		{"ctrl+a", devices.KeyCombo{Key: "a", Modifiers: []string{"control"}}},
		{"ctrl+shift+z", devices.KeyCombo{Key: "z", Modifiers: []string{"control", "shift"}}},
		// uppercase is the unshifted key, matching shortcut notation: CMD+A == ⌘a
		{"CMD+A", devices.KeyCombo{Key: "a", Modifiers: []string{"command"}}},
		{"alt+tab", devices.KeyCombo{Key: "tab", Modifiers: []string{"option"}}},
		{"meta+c", devices.KeyCombo{Key: "c", Modifiers: []string{"command"}}},
		{"backspace", devices.KeyCombo{Key: "backspace", Modifiers: []string{}}},
		{"delete", devices.KeyCombo{Key: "delete", Modifiers: []string{}}},
		{"shift++", devices.KeyCombo{Key: "+", Modifiers: []string{"shift"}}},
		{"+", devices.KeyCombo{Key: "+", Modifiers: []string{}}},
	}

	for _, test := range tests {
		combo, err := ParseKeyCombo(test.combo)
		if err != nil {
			t.Errorf("ParseKeyCombo(%q) returned error: %v", test.combo, err)
			continue
		}
		if !reflect.DeepEqual(combo, test.expected) {
			t.Errorf("ParseKeyCombo(%q) = %+v, expected %+v", test.combo, combo, test.expected)
		}
	}
}

func TestParseKeyComboErrors(t *testing.T) {
	invalidCombos := []string{
		"",
		"bogus+a",
		"a+cmd",
	}

	for _, combo := range invalidCombos {
		if _, err := ParseKeyCombo(combo); err == nil {
			t.Errorf("ParseKeyCombo(%q) should have returned an error", combo)
		}
	}
}
