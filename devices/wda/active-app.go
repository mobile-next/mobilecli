package wda

import (
	"encoding/json"
	"fmt"
)

// ActiveAppInfo represents information about the currently active application
type ActiveAppInfo struct {
	BundleID  string `json:"bundleId"`
	Name      string `json:"name"`
	ProcessID int    `json:"pid"`
}

func (c *WdaClient) GetActiveAppInfo() (*ActiveAppInfo, error) {
	result, err := c.CallRPC("device.apps.foreground", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	var info ActiveAppInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return nil, fmt.Errorf("failed to parse active app info: %w", err)
	}

	return &info, nil
}
