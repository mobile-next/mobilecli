package wda

import (
	"fmt"
)

func (c *WdaClient) GetWindowSize() (*WindowSize, error) {
	sessionId, err := c.GetOrCreateSession()
	if err != nil {
		return nil, err
	}

	response, err := c.GetEndpoint(fmt.Sprintf("session/%s/wda/screen", sessionId))
	if err != nil {
		return nil, err
	}

	windowSize := response["value"].(map[string]interface{})
	screenSize := windowSize["screenSize"].(map[string]interface{})

	return &WindowSize{
		Scale: int(windowSize["scale"].(float64)),
		ScreenSize: Size{
			Width:  int(screenSize["width"].(float64)),
			Height: int(screenSize["height"].(float64)),
		},
	}, nil
}
