package wda

import "fmt"

// ActiveAppInfo represents information about the currently active application
type ActiveAppInfo struct {
	BundleID string `json:"bundleId"`
	Name     string `json:"name"`
	ProcessID int   `json:"pid"`
}

// GetActiveAppInfo returns information about the currently active/foreground application
// This uses the /wda/activeAppInfo endpoint which doesn't require a session
func (c *WdaClient) GetActiveAppInfo() (*ActiveAppInfo, error) {
	response, err := c.GetEndpoint("wda/activeAppInfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	// extract value from response
	value, ok := response["value"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing or invalid 'value' field")
	}

	bundleID, _ := value["bundleId"].(string)
	name, _ := value["name"].(string)
	pid := 0
	if pidFloat, ok := value["pid"].(float64); ok {
		pid = int(pidFloat)
	}

	return &ActiveAppInfo{
		BundleID:  bundleID,
		Name:      name,
		ProcessID: pid,
	}, nil
}
