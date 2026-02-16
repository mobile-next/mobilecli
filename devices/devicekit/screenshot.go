package devicekit

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

func (c *Client) TakeScreenshot() ([]byte, error) {
	params := map[string]interface{}{
		"format":   "png",
		"deviceId": "",
	}

	result, err := c.call("device.screenshot", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse screenshot response: %w", err)
	}

	b64 := response.Data
	if idx := strings.Index(b64, ","); idx != -1 {
		b64 = b64[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode screenshot: %w", err)
	}

	return decoded, nil
}
