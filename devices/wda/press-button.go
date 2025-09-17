package wda

import (
	"fmt"
	"log"
)

func (c *WdaClient) PressButton(key string) error {
	buttonMap := map[string]string{
		"VOLUME_UP":   "volumeup",
		"VOLUME_DOWN": "volumedown",
		"HOME":        "home",
	}

	if key == "enter" {
		return c.SendKeys("\n")
	}

	translatedKey, exists := buttonMap[key]
	if !exists {
		return fmt.Errorf("unsupported button: %s", key)
	}

	return c.withSession(func(sessionId string) error {
		data := map[string]interface{}{
			"name": translatedKey,
		}

		_, err := c.PostEndpoint(fmt.Sprintf("session/%s/wda/pressButton", sessionId), data)
		if err != nil {
			return fmt.Errorf("failed to press button: %v", err)
		}

		log.Printf("press button response: %v", data)
		return nil
	})
}
