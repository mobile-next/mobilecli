package commands

import (
	"reflect"
	"testing"

	"github.com/mobile-next/mobilecli/devices/wda"
)

func TestParseKeyCombo(t *testing.T) {
	tests := []struct {
		combo    string
		expected wda.KeyCombo
	}{
		{"cmd+a", wda.KeyCombo{Key: "a", Modifiers: []string{"command"}}},
		{"ctrl+a", wda.KeyCombo{Key: "a", Modifiers: []string{"control"}}},
		{"ctrl+shift+z", wda.KeyCombo{Key: "z", Modifiers: []string{"control", "shift"}}},
		{"CMD+A", wda.KeyCombo{Key: "a", Modifiers: []string{"command"}}},
		{"alt+tab", wda.KeyCombo{Key: "tab", Modifiers: []string{"option"}}},
		{"meta+c", wda.KeyCombo{Key: "c", Modifiers: []string{"command"}}},
		{"backspace", wda.KeyCombo{Key: "backspace", Modifiers: []string{}}},
		{"delete", wda.KeyCombo{Key: "delete", Modifiers: []string{}}},
		{"shift++", wda.KeyCombo{Key: "+", Modifiers: []string{"shift"}}},
		{"+", wda.KeyCombo{Key: "+", Modifiers: []string{}}},
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
