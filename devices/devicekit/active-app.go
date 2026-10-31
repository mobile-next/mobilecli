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

// activeAppIdentityFields are the parts of the foreground app's description
// that cost the agent nothing to report.
var activeAppIdentityFields = []string{"bundleId", "pid"}

type activeAppRequest struct {
	Fields []string `json:"fields,omitempty"`
}

func (c *DeviceKitClient) GetActiveAppInfo() (*ActiveAppInfo, error) {
	return c.getActiveAppInfo(activeAppRequest{})
}

// GetActiveAppIdentity reports only the foreground app's bundle id and pid.
// Its name and view controller are left empty: the agent finds the view
// controller by snapshotting the hierarchy, which on an app with a large one
// costs as much as a UI dump, and callers that only need to know which app is
// in front should not pay for it.
func (c *DeviceKitClient) GetActiveAppIdentity() (*ActiveAppInfo, error) {
	return c.getActiveAppInfo(activeAppRequest{Fields: activeAppIdentityFields})
}

func (c *DeviceKitClient) getActiveAppInfo(request activeAppRequest) (*ActiveAppInfo, error) {
	// older agents ignore the fields and report everything
	result, err := c.CallRPC("device.apps.foreground", request)
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	var info ActiveAppInfo
	if err := json.Unmarshal(result, &info); err != nil {
		return nil, fmt.Errorf("failed to parse active app info: %w", err)
	}

	return &info, nil
}
