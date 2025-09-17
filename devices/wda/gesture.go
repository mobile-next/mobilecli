package wda

import (
	"encoding/json"
	"fmt"
)

func (c *WdaClient) Gesture(actions []TapAction) error {
	return c.withSession(func(sessionId string) error {
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

		_, err := c.PostEndpoint(fmt.Sprintf("session/%s/actions", sessionId), data)
		return err
	})
}

func (c *WdaClient) GestureFromJSON(jsonData []byte) error {
	var actions []TapAction
	if err := json.Unmarshal(jsonData, &actions); err != nil {
		return fmt.Errorf("failed to parse gesture actions: %v", err)
	}

	return c.Gesture(actions)
}
