package wda

import (
	"fmt"
)

func (c *WdaClient) LongPress(x, y int) error {

	sessionId, err := c.CreateSession()
	if err != nil {
		return err
	}

	defer c.DeleteSession(sessionId)

	data := ActionsRequest{
		Actions: []Pointer{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: PointerParameters{
					PointerType: "touch",
				},
				Actions: []TapAction{
					{Type: "pointerMove", Duration: 0, X: x, Y: y},
					{Type: "pointerDown", Button: 0},
					{Type: "pause", Duration: 500},
					{Type: "pointerUp", Button: 0},
				},
			},
		},
	}

	_, err = c.PostEndpoint(fmt.Sprintf("session/%s/actions", sessionId), data)
	if err != nil {
		return err
	}
	return nil
}