package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// requireCapability asserts that a device implements the optional capability
// T, such as devices.AppManager or devices.FileSystem.
func requireCapability[T any](device devices.ControllableDevice, capability string) (T, error) {
	capable, ok := device.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("%s is not supported on %s %s devices", capability, device.Platform(), device.DeviceType())
	}

	return capable, nil
}

// findDeviceWith resolves a device by ID (or auto-selects one) and asserts it
// implements the capability T.
func findDeviceWith[T any](deviceID, capability string) (T, devices.ControllableDevice, error) {
	device, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		var zero T
		return zero, nil, err
	}

	capable, err := requireCapability[T](device, capability)
	if err != nil {
		return capable, nil, err
	}

	return capable, device, nil
}

func findAppManager(deviceID string) (devices.AppManager, devices.ControllableDevice, error) {
	return findDeviceWith[devices.AppManager](deviceID, "app management")
}

func findCrashReporter(deviceID string) (devices.CrashReporter, devices.ControllableDevice, error) {
	return findDeviceWith[devices.CrashReporter](deviceID, "crash reports")
}

func findFileSystem(deviceID string) (devices.FileSystem, devices.ControllableDevice, error) {
	return findDeviceWith[devices.FileSystem](deviceID, "file access")
}
