package wda

import "fmt"

// siriRemoteButtons are the tvOS Siri Remote buttons handled by the tvOS
// device.io.button handler. They are unsupported on non-tvOS platforms.
var siriRemoteButtons = map[string]bool{
	"UP":         true,
	"DOWN":       true,
	"LEFT":       true,
	"RIGHT":      true,
	"SELECT":     true,
	"MENU":       true,
	"PLAY_PAUSE": true,
}

// iosOnlyButtons are iPhone/iPad-only hardware buttons with no Siri Remote
// equivalent; they are rejected on tvOS.
var iosOnlyButtons = map[string]bool{
	"HOME":        true,
	"LOCK":        true,
	"VOLUME_UP":   true,
	"VOLUME_DOWN": true,
}

// ValidateButtonForPlatform enforces per-platform button gating: tvOS accepts the
// Siri Remote buttons and rejects iPhone-only buttons, while non-tvOS platforms
// reject Siri Remote buttons. Unknown buttons fall through to the button map.
func ValidateButtonForPlatform(platform, key string) error {
	if platform == "tvos" {
		if iosOnlyButtons[key] {
			return fmt.Errorf("unsupported on tvOS: %s", key)
		}
		return nil
	}
	if siriRemoteButtons[key] {
		return fmt.Errorf("unsupported on %s: %s", platform, key)
	}
	return nil
}

func (c *WdaClient) PressButton(key string) error {
	buttonMap := map[string]string{
		"VOLUME_UP":   "volumeUp",
		"VOLUME_DOWN": "volumeDown",
		"HOME":        "home",
		"LOCK":        "lock",
		// tvOS Siri Remote buttons (handled by the tvOS device.io.button handler).
		"UP":         "up",
		"DOWN":       "down",
		"LEFT":       "left",
		"RIGHT":      "right",
		"SELECT":     "select",
		"MENU":       "menu",
		"PLAY_PAUSE": "playPause",
	}

	if key == "ENTER" {
		return c.SendKeys("\n")
	}

	translatedKey, exists := buttonMap[key]
	if !exists {
		return fmt.Errorf("unsupported button: %s", key)
	}

	params := map[string]string{
		"button": translatedKey,
	}

	_, err := c.CallRPC("device.io.button", params)
	return err
}
