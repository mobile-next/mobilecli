package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// CommandResponse represents a standardized response format for all commands
type CommandResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(data interface{}) *CommandResponse {
	return &CommandResponse{
		Status: "ok",
		Data:   data,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(err error) *CommandResponse {
	return &CommandResponse{
		Status: "error",
		Error:  err.Error(),
	}
}

// DeviceCache provides a simple cache for devices to avoid repeated lookups
var deviceCache = make(map[string]devices.ControllableDevice)

// FindDevice finds a device by ID, using cache when possible
func FindDevice(deviceID string) (devices.ControllableDevice, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID is required")
	}

	// Check cache first
	if device, exists := deviceCache[deviceID]; exists {
		return device, nil
	}

	// Get all devices and find the one we want
	allDevices, err := devices.GetAllControllableDevices()
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	for _, d := range allDevices {
		if d.ID() == deviceID {
			deviceCache[deviceID] = d
			return d, nil
		}
	}

	return nil, fmt.Errorf("device not found: %s", deviceID)
}