package devicekit

import (
	"encoding/json"
	"fmt"
)

type clipboardText struct {
	Text string `json:"text"`
}

func (c *DeviceKitClient) GetClipboard() (string, error) {
	result, err := c.CallRPC("device.clipboard.get", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get clipboard: %w", err)
	}

	var clipboard clipboardText
	if err := json.Unmarshal(result, &clipboard); err != nil {
		return "", fmt.Errorf("failed to parse clipboard: %w", err)
	}

	return clipboard.Text, nil
}

func (c *DeviceKitClient) SetClipboard(text string) error {
	_, err := c.CallRPC("device.clipboard.set", map[string]any{"text": text})
	if err != nil {
		return fmt.Errorf("failed to set clipboard: %w", err)
	}

	return nil
}
