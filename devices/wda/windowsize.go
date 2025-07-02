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

type windowSizeResponse struct {
	Value WindowSize `json:"value"`
}

func GetWindowSize() (*WindowSize, error) {
	sessionId, err := CreateSession()
	if err != nil {
		return nil, err
	}

	defer DeleteSession(sessionId)

	response, err := GetWebDriverAgentEndpoint(fmt.Sprintf("session/%s/wda/screen", sessionId))
	if err != nil {
		return nil, err
	}

	// log.Printf("response: %v", response["value"])

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
