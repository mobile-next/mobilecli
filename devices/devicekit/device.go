package devicekit

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/types"
)

func (c *Client) GetOrientation() (string, error) {
	params := map[string]interface{}{
		"deviceId": "",
	}

	result, err := c.call("device.io.orientation.get", params)
	if err != nil {
		return "", fmt.Errorf("failed to get orientation: %w", err)
	}

	var response struct {
		Orientation string `json:"orientation"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", fmt.Errorf("failed to parse orientation response: %w", err)
	}

	switch strings.ToUpper(response.Orientation) {
	case "PORTRAIT", "UIA_DEVICE_ORIENTATION_PORTRAIT_UPSIDEDOWN":
		return "portrait", nil
	case "LANDSCAPE", "UIA_DEVICE_ORIENTATION_LANDSCAPERIGHT":
		return "landscape", nil
	default:
		return "portrait", nil
	}
}

func (c *Client) SetOrientation(orientation string) error {
	if orientation != "portrait" && orientation != "landscape" {
		return fmt.Errorf("invalid orientation value '%s', must be 'portrait' or 'landscape'", orientation)
	}

	dkOrientation := "PORTRAIT"
	if orientation == "landscape" {
		dkOrientation = "LANDSCAPE"
	}

	params := map[string]interface{}{
		"orientation": dkOrientation,
		"deviceId":    "",
	}

	_, err := c.call("device.io.orientation.set", params)
	if err != nil {
		return fmt.Errorf("failed to set orientation: %w", err)
	}

	return nil
}

func (c *Client) GetWindowSize() (*types.WindowSize, error) {
	params := map[string]interface{}{
		"deviceId": "",
	}

	result, err := c.call("device.info", params)
	if err != nil {
		return nil, err
	}

	var response struct {
		ScreenSize struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"screenSize"`
		Scale float64 `json:"scale"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse device info response: %w", err)
	}

	return &types.WindowSize{
		Scale: int(response.Scale),
		ScreenSize: types.Size{
			Width:  int(response.ScreenSize.Width),
			Height: int(response.ScreenSize.Height),
		},
	}, nil
}

func (c *Client) GetForegroundApp() (*types.ActiveAppInfo, error) {
	params := map[string]interface{}{
		"deviceId": "",
	}

	result, err := c.call("device.apps.foreground", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreground app: %w", err)
	}

	var response struct {
		BundleID string  `json:"bundleId"`
		Name     string  `json:"name"`
		PID      float64 `json:"pid"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse foreground app response: %w", err)
	}

	return &types.ActiveAppInfo{
		BundleID:  response.BundleID,
		Name:      response.Name,
		ProcessID: int(response.PID),
	}, nil
}

func (c *Client) LaunchApp(bundleID string) error {
	params := map[string]interface{}{
		"bundleId": bundleID,
		"deviceId": "",
	}
	_, err := c.call("device.apps.launch", params)
	return err
}

func (c *Client) TerminateApp(bundleID string) error {
	params := map[string]interface{}{
		"bundleId": bundleID,
		"deviceId": "",
	}
	_, err := c.call("device.apps.terminate", params)
	return err
}
