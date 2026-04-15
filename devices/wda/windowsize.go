package wda

import (
	"encoding/json"
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
	result, err := c.CallRPC("device.info", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	var ws WindowSize
	if err := json.Unmarshal(result, &ws); err != nil {
		return nil, fmt.Errorf("failed to parse device info: %w", err)
	}

	return &ws, nil
}
