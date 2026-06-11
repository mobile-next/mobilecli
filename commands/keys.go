package commands

import (
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/devices/wda"
)

// KeysRequest represents the parameters for a keys command
type KeysRequest struct {
	DeviceID string   `json:"deviceId"`
	Keys     []string `json:"keys"`
}

// modifierAliases maps user-facing modifier names to their canonical form
var modifierAliases = map[string]string{
	"cmd":     "command",
	"command": "command",
	"meta":    "command",
	"ctrl":    "control",
	"control": "control",
	"alt":     "option",
	"opt":     "option",
	"option":  "option",
	"shift":   "shift",
	"fn":      "fn",
}

// ParseKeyCombo parses a key combo string like "cmd+a", "ctrl+shift+z" or
// "backspace" into a key and its canonical modifiers.
func ParseKeyCombo(combo string) (wda.KeyCombo, error) {
	if combo == "" {
		return wda.KeyCombo{}, fmt.Errorf("key combo cannot be empty")
	}

	parts := strings.Split(combo, "+")

	key := parts[len(parts)-1]
	modifierParts := parts[:len(parts)-1]

	// a trailing '+' means the key itself is '+', e.g. "shift++"
	if key == "" {
		key = "+"
		if len(modifierParts) > 0 {
			modifierParts = modifierParts[:len(modifierParts)-1]
		}
	}

	modifiers := make([]string, 0, len(modifierParts))
	for _, part := range modifierParts {
		modifier, ok := modifierAliases[strings.ToLower(part)]
		if !ok {
			return wda.KeyCombo{}, fmt.Errorf("unsupported modifier '%s' in key combo '%s'", part, combo)
		}
		modifiers = append(modifiers, modifier)
	}

	return wda.KeyCombo{
		Key:       strings.ToLower(key),
		Modifiers: modifiers,
	}, nil
}

// KeysCommand presses one or more key combos on the specified device. All
// combos are validated before any key is pressed.
func KeysCommand(req KeysRequest) *CommandResponse {
	if len(req.Keys) == 0 {
		return NewErrorResponse(fmt.Errorf("at least one key combo is required"))
	}

	combos := make([]wda.KeyCombo, len(req.Keys))
	for i, comboStr := range req.Keys {
		combo, err := ParseKeyCombo(comboStr)
		if err != nil {
			return NewErrorResponse(err)
		}
		combos[i] = combo
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.PressKeys(combos)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to press keys on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Pressed keys on device %s", targetDevice.ID()),
	})
}
