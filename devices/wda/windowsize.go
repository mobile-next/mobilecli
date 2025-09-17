package wda

import (
	"fmt"
)

type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type WindowSize struct {
	Scale      int  `json:"scale"`
	ScreenSize Size `json:"screenSize"`
}

func (c *WdaClient) GetWindowSize() (*WindowSize, error) {
	var windowSize *WindowSize
	err := c.withSession(func(sessionId string) error {
		response, err := c.GetEndpoint(fmt.Sprintf("session/%s/wda/screen", sessionId))
		if err != nil {
			return err
		}

		value := response["value"].(map[string]interface{})
		screenSize := value["screenSize"].(map[string]interface{})

		windowSize = &WindowSize{
			Scale: int(value["scale"].(float64)),
			ScreenSize: Size{
				Width:  int(screenSize["width"].(float64)),
				Height: int(screenSize["height"].(float64)),
			},
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return windowSize, nil
}
