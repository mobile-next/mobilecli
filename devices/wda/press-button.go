package wda

import (
	"fmt"
	"log"
)

func PressButton(key string) error {
	sessionId, err := CreateSession()
	if err != nil {
		return fmt.Errorf("SimulatorDevice: failed to create session: %v", err)
	}

	data := map[string]interface{}{
		"name": key,
	}

	_, err = PostWebDriverAgentEndpoint(fmt.Sprintf("session/%s/wda/pressButton", sessionId), data)
	if err != nil {
		return fmt.Errorf("SimulatorDevice: failed to press button: %v", err)
	}
	log.Printf("SimulatorDevice: press button response: %v", data)

	return nil
}
