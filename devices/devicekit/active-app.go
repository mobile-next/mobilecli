package devicekit

import (
	"encoding/json"
	"fmt"
)

// ActiveAppInfo represents information about the currently active application
type ActiveAppInfo struct {
	BundleID  string `json:"bundleId"`
	Name      string `json:"name"`
	ProcessID int    `json:"pid"`
	// ViewController is the class name of the controller presenting the current
	// screen. Empty when the agent predates the field or has nothing to report.
	ViewController string `json:"viewController"`
}

func (c *DeviceKitClient) GetActiveAppInfo() (*ActiveAppInfo, error) {
	return c.getActiveAppInfo(true)
}

// GetActiveAppIdentity names the foreground app without asking which view
// controller is on screen. The agent finds that by snapshotting the hierarchy,
// which on an app with a large one costs as much as a UI dump; callers that
// only need to know which app is in front should not pay for it.
func (c *DeviceKitClient) GetActiveAppIdentity() (*ActiveAppInfo, error) {
	return c.getActiveAppInfo(false)
}

func (c *DeviceKitClient) getActiveAppInfo(withViewController bool) (*ActiveAppInfo, error) {
	// older agents ignore the parameter and always report the view controller
	result, err := c.CallRPC("device.apps.foreground", map[string]bool{"viewController": withViewController})
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	var info ActiveAppInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return nil, fmt.Errorf("failed to parse active app info: %w", err)
	}

	return &info, nil
}
