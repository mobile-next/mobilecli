package commands

import (
	"fmt"
	"strings"

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

// FindDeviceOrAutoSelect finds a device by ID, or auto-selects if deviceID is empty
func FindDeviceOrAutoSelect(deviceID string) (devices.ControllableDevice, error) {
	// if deviceID is provided, use existing logic
	if deviceID != "" {
		return FindDevice(deviceID)
	}

	// Get all devices for auto-selection
	allDevices, err := devices.GetAllControllableDevices()
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	// filter to only online devices for auto-selection
	var onlineDevices []devices.ControllableDevice
	for _, d := range allDevices {
		if d.State() == "online" {
			onlineDevices = append(onlineDevices, d)
		}
	}

	if len(onlineDevices) == 0 {
		return nil, fmt.Errorf("no online devices found")
	}

	if len(onlineDevices) == 1 {
		device := onlineDevices[0]
		// Cache the device for future use
		deviceCache[device.ID()] = device
		return device, nil
	}

	err = fmt.Errorf("multiple devices found (%d), please specify --device with one of: %s", len(onlineDevices), getDeviceIDList(onlineDevices))
	return nil, err
}

// getDeviceIDList returns a comma-separated list of device IDs for error messages
func getDeviceIDList(devices []devices.ControllableDevice) string {
	var ids []string
	for _, d := range devices {
		ids = append(ids, d.ID())
	}
	return fmt.Sprintf("[%s]", strings.Join(ids, ", "))
}
