package devicekit

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// enterButton is not a hardware button: it is typed as a newline.
const enterButton = "ENTER"

// buttons maps the button names `io button` accepts to DeviceKit button names.
var buttons = map[string]string{
	"VOLUME_UP":   "volumeUp",
	"VOLUME_DOWN": "volumeDown",
	"HOME":        "home",
	"LOCK":        "lock",
	"POWER":       "lock", // alias of LOCK, the name Android started with
}

// ButtonNames returns the button names an iOS device accepts, sorted.
func ButtonNames() []string {
	names := append(slices.Collect(maps.Keys(buttons)), enterButton)
	slices.Sort(names)
	return names
}

func translateButton(key string) (string, error) {
	translatedKey, exists := buttons[key]
	if !exists {
		return "", fmt.Errorf("unsupported button: %s (supported: %s)", key, strings.Join(ButtonNames(), ", "))
	}
	return translatedKey, nil
}

func (c *DeviceKitClient) PressButton(key string) error {
	if key == enterButton {
		return c.SendKeys("\n")
	}

	translatedKey, err := translateButton(key)
	if err != nil {
		return err
	}

	params := map[string]string{
		"button": translatedKey,
	}

	_, err = c.CallRPC("device.io.button", params)
	return err
}
