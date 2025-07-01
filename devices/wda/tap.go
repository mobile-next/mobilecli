package wda

import (
	"fmt"
)

func Tap(x, y int) error {

	sessionId, err := CreateSession()
	if err != nil {
		return err
	}

	defer DeleteSession(sessionId)

	data := map[string]interface{}{
		"actions": []map[string]interface{}{
			{
				"type": "pointer",
				"id":   "finger1",
				"parameters": map[string]interface{}{
					"pointerType": "touch",
				},
				"actions": []map[string]interface{}{
					{"type": "pointerMove", "duration": 0, "x": x, "y": y},
					{"type": "pointerDown", "button": 0},
					{"type": "pause", "duration": 100},
					{"type": "pointerUp", "button": 0},
				},
			},
		},
	}

	_, err = PostWebDriverAgentEndpoint(fmt.Sprintf("session/%s/actions", sessionId), data)
	if err != nil {
		return err
	}
	return nil
}
