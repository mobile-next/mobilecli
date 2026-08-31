package devicekit

import (
	"encoding/json"
	"fmt"
)

func (c *DeviceKitClient) GetOrientation() (string, error) {
	result, err := c.CallRPC("device.io.orientation.get", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get orientation: %w", err)
	}

	var response struct {
		Orientation string `json:"orientation"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("failed to parse orientation response: %w", err)
	}

	switch response.Orientation {
	case "PORTRAIT":
		return "portrait", nil
	case "LANDSCAPE":
		return "landscape", nil
	default:
		return "portrait", nil
	}
}

func (c *DeviceKitClient) SetOrientation(orientation string) error {
	if orientation != "portrait" && orientation != "landscape" {
		return fmt.Errorf("invalid orientation value '%s', must be 'portrait' or 'landscape'", orientation)
	}

	wdaOrientation := "PORTRAIT"
	if orientation == "landscape" {
		wdaOrientation = "LANDSCAPE"
	}

	params := map[string]string{
		"orientation": wdaOrientation,
	}

	_, err := c.CallRPC("device.io.orientation.set", params)
	return err
}
