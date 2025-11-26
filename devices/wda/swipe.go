package wda

import (
	"fmt"
)

func (c *WdaClient) Swipe(x1, y1, x2, y2 int) error {

	sessionId, err := c.GetOrCreateSession()
	if err != nil {
		return err
	}

	data := ActionsRequest{
		Actions: []Pointer{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: PointerParameters{
					PointerType: "touch",
				},
				Actions: []TapAction{
					{Type: "pointerMove", Duration: 0, X: x1, Y: y1},
					{Type: "pointerDown", Button: 0},
					{Type: "pointerMove", Duration: 1000, X: x2, Y: y2},
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
