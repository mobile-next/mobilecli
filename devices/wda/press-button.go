package wda

import "fmt"

func (c *WdaClient) PressButton(key string) error {
	buttonMap := map[string]string{
		"VOLUME_UP":   "volumeUp",
		"VOLUME_DOWN": "volumeDown",
		"HOME":        "home",
		"LOCK":        "lock",
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
