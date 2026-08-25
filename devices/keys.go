package devices

import "github.com/mobile-next/mobilecli/devices/devicekit"

// KeyCombo is a single key press with optional modifier keys. It is the
// currency of ControllableDevice.PressKeys; each device adapter maps it to its
// platform representation.
type KeyCombo struct {
	Key       string
	Modifiers []string
}

// toWdaKeyCombos maps the port type to the WebDriverAgent wire DTO.
func toWdaKeyCombos(combos []KeyCombo) []devicekit.KeyCombo {
	out := make([]devicekit.KeyCombo, len(combos))
	for i, c := range combos {
		out[i] = devicekit.KeyCombo{Key: c.Key, Modifiers: c.Modifiers}
	}
	return out
}
