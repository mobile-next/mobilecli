package commands

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

// CommandResponse represents a standardized response format for all commands
type CommandResponse struct {
	Status string      `json:"status"`
	Data   any `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(data any) *CommandResponse {
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

var (
	mu         sync.RWMutex
	fleetToken string
)

func SetFleetConfig(token string) {
	mu.Lock()
	fleetToken = token
	mu.Unlock()
}

func GetFleetToken() string {
	mu.RLock()
	defer mu.RUnlock()
	return fleetToken
}

func getRemoteControllableDevices() []devices.ControllableDevice {
	token := GetFleetToken()
	if token == "" {
		return nil
	}

	remoteInfos, err := FetchRemoteDevices(token)
	if err != nil {
		utils.Verbose("failed to fetch remote devices: %v", err)
		return nil
	}

	var result []devices.ControllableDevice
	for _, info := range remoteInfos {
		result = append(result, devices.NewRemoteDevice(info, token))
	}

	return result
}

// DeviceCache provides a simple cache for devices to avoid repeated lookups
var deviceCache = make(map[string]devices.ControllableDevice)

// shutdownHook holds the shutdown hook for resource cleanup tracking.
// It is set once at application startup via SetShutdownHook and used by commands
// to register cleanup functions for graceful shutdown.
var shutdownHook *devices.ShutdownHook

// SetShutdownHook sets the global shutdown hook for resource cleanup.
// This should be called once at application startup (main.go or server.go).
// The hook is used to register cleanup functions that will be called
// during graceful shutdown (SIGINT/SIGTERM).
func SetShutdownHook(hook *devices.ShutdownHook) {
	mu.Lock()
	shutdownHook = hook
	mu.Unlock()
}

// GetShutdownHook returns the current shutdown hook.
// Returns nil if SetShutdownHook has not been called yet.
// Commands use this to register cleanup functions.
func GetShutdownHook() *devices.ShutdownHook {
	mu.RLock()
	defer mu.RUnlock()
	return shutdownHook
}

// FindDevice finds a device by ID, using cache when possible
func FindDevice(deviceID string) (devices.ControllableDevice, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID is required")
	}

	// Check cache first
	mu.RLock()
	device, exists := deviceCache[deviceID]
	mu.RUnlock()
	if exists {
		return device, nil
	}

	// get all devices including offline ones and find the one we want
	allDevices, err := devices.GetAllControllableDevices(true)
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %w", err)
	}

	// append remote devices
	allDevices = append(allDevices, getRemoteControllableDevices()...)

	for _, d := range allDevices {
		if d.ID() == deviceID {
			mu.Lock()
			deviceCache[deviceID] = d
			mu.Unlock()
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

	// get all devices for auto-selection
	allDevices, err := devices.GetAllControllableDevices(false)
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %w", err)
	}

	// append remote devices
	allDevices = append(allDevices, getRemoteControllableDevices()...)

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

	if len(onlineDevices) > 1 {
		err = fmt.Errorf("multiple devices found (%d), please specify --device with one of: %s", len(onlineDevices), getDeviceIDList(onlineDevices))
		return nil, err
	}

	// exactly 1 online device - check cache first to reuse existing instance
	deviceID = onlineDevices[0].ID()
	mu.RLock()
	cachedDevice, exists := deviceCache[deviceID]
	mu.RUnlock()
	if exists {
		return cachedDevice, nil
	}

	// not in cache, use the new device instance and cache it
	device := onlineDevices[0]
	mu.Lock()
	deviceCache[device.ID()] = device
	mu.Unlock()
	return device, nil
}

// getDeviceIDList returns a comma-separated list of device IDs for error messages
func getDeviceIDList(devices []devices.ControllableDevice) string {
	var ids []string
	for _, d := range devices {
		ids = append(ids, d.ID())
	}
	return fmt.Sprintf("[%s]", strings.Join(ids, ", "))
}
