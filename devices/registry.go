package devices

import (
	"sync"

	"github.com/mobile-next/mobilecli/utils"
)

type DeviceRegistry struct {
	mu      sync.RWMutex
	devices map[string]*IOSDevice
}

// NewDeviceRegistry creates a new device registry instance
func NewDeviceRegistry() *DeviceRegistry {
	return &DeviceRegistry{
		devices: make(map[string]*IOSDevice),
	}
}

// Register adds a device to the registry for cleanup tracking
func (r *DeviceRegistry) Register(device *IOSDevice) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.devices[device.Udid] = device
}

// CleanupAll gracefully cleans up all registered devices
func (r *DeviceRegistry) CleanupAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.devices) == 0 {
		return
	}

	for udid, device := range r.devices {
		if err := device.Cleanup(); err != nil {
			utils.Verbose("Error cleaning up device %s: %v", udid, err)
		}
	}

	// clear the registry
	r.devices = make(map[string]*IOSDevice)
}
