package wda

import (
	"fmt"
)

func (c *WdaClient) Tap(x, y int) error {
	return c.withSession(func(sessionId string) error {
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
						{Type: "pause", Duration: 100},
						{Type: "pointerUp", Button: 0},
					},
				},
			},
		}

		_, err := c.PostEndpoint(fmt.Sprintf("session/%s/actions", sessionId), data)
		return err
	})
}
