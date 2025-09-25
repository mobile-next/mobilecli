package wda

import (
	"fmt"
)

// GetOrientation gets the current device orientation
func (c *WdaClient) GetOrientation() (string, error) {
	sessionId, err := c.CreateSession()
	if err != nil {
		return "", err
	}
	defer c.DeleteSession(sessionId)

	endpoint := fmt.Sprintf("session/%s/orientation", sessionId)
	response, err := c.GetEndpoint(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to get orientation: %v", err)
	}

	value, ok := response["value"].(string)
	if !ok {
		return "", fmt.Errorf("invalid orientation response format")
	}

	// convert WDA orientation to simplified format
	switch value {
	case "PORTRAIT":
		return "portrait", nil
	case "LANDSCAPE", "UIA_DEVICE_ORIENTATION_LANDSCAPELEFT":
		return "landscape", nil
	case "PORTRAIT_UPSIDEDOWN", "UIA_DEVICE_ORIENTATION_PORTRAIT_UPSIDEDOWN":
		return "portrait", nil
	case "LANDSCAPERIGHT", "UIA_DEVICE_ORIENTATION_LANDSCAPERIGHT":
		return "landscape", nil
	default:
		return "portrait", nil // default to portrait
	}
}

// SetOrientation sets the device orientation
func (c *WdaClient) SetOrientation(orientation string) error {
	if orientation != "portrait" && orientation != "landscape" {
		return fmt.Errorf("invalid orientation value '%s', must be 'portrait' or 'landscape'", orientation)
	}

	sessionId, err := c.CreateSession()
	if err != nil {
		return err
	}
	defer c.DeleteSession(sessionId)

	// convert simplified orientation to WDA format
	wdaOrientation := "PORTRAIT"
	if orientation == "landscape" {
		wdaOrientation = "LANDSCAPE"
	}

	endpoint := fmt.Sprintf("session/%s/orientation", sessionId)
	data := map[string]interface{}{
		"orientation": wdaOrientation,
	}

	_, err = c.PostEndpoint(endpoint, data)
	if err != nil {
		return fmt.Errorf("failed to set orientation: %v", err)
	}

	return nil
}