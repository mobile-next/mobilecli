package wda

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

func (c *WdaClient) TakeScreenshot() ([]byte, error) {
	params := map[string]string{
		"format": "png",
	}

	result, err := c.CallRPC("device.screenshot", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse screenshot response: %w", err)
	}

	// strip data URI prefix (e.g. "data:image/png;base64,")
	b64Data := response.Data
	if idx := strings.Index(b64Data, ","); idx != -1 {
		b64Data = b64Data[idx+1:]
	}

	return base64.StdEncoding.DecodeString(b64Data)
}
