package wda

import (
	"encoding/json"
	"fmt"
)

func Gesture(actions []TapAction) error {
	sessionId, err := CreateSession()
	if err != nil {
		return err
	}

	defer DeleteSession(sessionId)

	data := ActionsRequest{
		Actions: []Pointer{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: PointerParameters{
					PointerType: "touch",
				},
				Actions: actions,
			},
		},
	}

	_, err = PostWebDriverAgentEndpoint(fmt.Sprintf("session/%s/actions", sessionId), data)
	if err != nil {
		return err
	}
	return nil
}

func GestureFromJSON(jsonData []byte) error {
	var actions []TapAction
	if err := json.Unmarshal(jsonData, &actions); err != nil {
		return fmt.Errorf("failed to parse gesture actions: %v", err)
	}

	return Gesture(actions)
}
