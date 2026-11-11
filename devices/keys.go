package devices

import "github.com/mobile-next/mobilecli/devices/devicekit"

// KeyCombo is a single key press with optional modifier keys. It is the
// currency of ControllableDevice.PressKeys; each device adapter maps it to its
// platform representation.
type KeyCombo struct {
	Key       string
	Modifiers []string
}

// toDeviceKitKeyCombos maps the port type to the DeviceKit agent wire DTO.
func toDeviceKitKeyCombos(combos []KeyCombo) []devicekit.KeyCombo {
	out := make([]devicekit.KeyCombo, len(combos))
	for i, c := range combos {
		// the agent rejects "modifiers": null, so a combo without modifiers sends []
		out[i] = devicekit.KeyCombo{Key: c.Key, Modifiers: append([]string{}, c.Modifiers...)}
	}
	return out
}
