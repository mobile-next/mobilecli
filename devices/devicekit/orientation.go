package devicekit

import (
	"encoding/json"
	"fmt"
)

const (
	OrientationPortrait  = "portrait"
	OrientationLandscape = "landscape"
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
		return OrientationPortrait, nil
	case "LANDSCAPE":
		return OrientationLandscape, nil
	default:
		return OrientationPortrait, nil
	}
}

func (c *DeviceKitClient) SetOrientation(orientation string) error {
	if orientation != OrientationPortrait && orientation != OrientationLandscape {
		return fmt.Errorf("invalid orientation value '%s', must be 'portrait' or 'landscape'", orientation)
	}

	wdaOrientation := "PORTRAIT"
	if orientation == OrientationLandscape {
		wdaOrientation = "LANDSCAPE"
	}

	params := map[string]string{
		"orientation": wdaOrientation,
	}

	_, err := c.CallRPC("device.io.orientation.set", params)
	return err
}
