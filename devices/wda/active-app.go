package wda

import (
	"encoding/json"
	"fmt"
)

// activeAppValue represents the value field from the WDA active app response
type activeAppValue struct {
	BundleID string  `json:"bundleId"`
	Name     string  `json:"name"`
	PID      float64 `json:"pid"`
}

// activeAppResponse represents the WDA API response for active app info
type activeAppResponse struct {
	Value activeAppValue `json:"value"`
}

// ActiveAppInfo represents information about the currently active application
type ActiveAppInfo struct {
	BundleID  string `json:"bundleId"`
	Name      string `json:"name"`
	ProcessID int    `json:"pid"`
}

// GetActiveAppInfo returns information about the currently active/foreground application
// This uses the /wda/activeAppInfo endpoint which doesn't require a session
func (c *WdaClient) GetActiveAppInfo() (*ActiveAppInfo, error) {
	responseMap, err := c.GetEndpoint("wda/activeAppInfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	// marshal the response back to json and unmarshal into typed struct
	jsonData, err := json.Marshal(responseMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var response activeAppResponse
	if err := json.Unmarshal(jsonData, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &ActiveAppInfo{
		BundleID:  response.Value.BundleID,
		Name:      response.Value.Name,
		ProcessID: int(response.Value.PID),
	}, nil
}
