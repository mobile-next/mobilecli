package wda

import (
	"encoding/json"
	"fmt"
)

// Focus selects an on-screen element by accessibility identifier and/or label and
// drives Siri Remote focus to it via the tvOS DeviceKit device.io.focus handler.
// At least one of identifier or label must be provided. It returns the focused
// element as decoded JSON.
func (c *WdaClient) Focus(identifier, label string) (any, error) {
	if identifier == "" && label == "" {
		return nil, fmt.Errorf("focus requires at least one of identifier or label")
	}

	params := map[string]string{}
	if identifier != "" {
		params["identifier"] = identifier
	}
	if label != "" {
		params["label"] = label
	}

	result, err := c.CallRPC("device.io.focus", params)
	if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal(result, &value); err != nil {
		return nil, fmt.Errorf("failed to parse focus result: %w", err)
	}
	return value, nil
}
