package devices

import (
	"sync"

	"github.com/mobile-next/mobilecli/utils"
)

type DeviceRegistry struct {
	mu      sync.RWMutex
	devices map[string]*IOSDevice
}

var globalRegistry = &DeviceRegistry{
	devices: make(map[string]*IOSDevice),
}

// RegisterDevice adds a device to the global registry for cleanup tracking
func RegisterDevice(device *IOSDevice) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.devices[device.Udid] = device
}

// UnregisterDevice removes a device from the global registry
func UnregisterDevice(udid string) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	delete(globalRegistry.devices, udid)
	utils.Verbose("Unregistered device %s from cleanup tracking", udid)
}

// CleanupAllDevices gracefully cleans up all registered devices
func CleanupAllDevices() {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	if len(globalRegistry.devices) == 0 {
		return
	}

	for udid, device := range globalRegistry.devices {
		if err := device.Cleanup(); err != nil {
			utils.Verbose("Error cleaning up device %s: %v", udid, err)
		}
	}

	// clear the registry
	globalRegistry.devices = make(map[string]*IOSDevice)
}
