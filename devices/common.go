package devices

import (
	"fmt"
	"log"
	"strings"
)

// ControllableDevice defines common operations that can be performed on any device (real or simulated).
type ControllableDevice interface {
	ID() string
	Name() string
	Platform() string   // e.g., "ios", "android"
	DeviceType() string // e.g., "real", "simulator", "emulator"

	TakeScreenshot() ([]byte, error)
	Reboot() error
	Tap(x, y int) error
	StartAgent() error
	SendKeys(text string) error
	PressButton(key string) error
	LaunchApp(bundleID string) error
	TerminateApp(bundleID string) error
}

// GetAllControllableDevices aggregates all known devices (iOS, Android, Simulators)
// and returns them as a slice of ControllableDevice.
func GetAllControllableDevices() ([]ControllableDevice, error) {
	var allDevices []ControllableDevice
	var errs []error

	// Get Android devices
	androidDevices, err := GetAndroidDevices() // Assumes this now returns []ControllableDevice
	if err != nil {
		// Log or collect error, decide if it's fatal or if we continue
		log.Printf("Warning: Failed to get Android devices: %v", err)
		errs = append(errs, fmt.Errorf("android: %w", err))
	} else {
		allDevices = append(allDevices, androidDevices...)
	}

	// Get iOS real devices
	iosDeviceEntries, err := ListIOSDevices() // This returns []ios.DeviceEntry
	if err != nil {
		log.Printf("Warning: Failed to get iOS real devices: %v", err)
		errs = append(errs, fmt.Errorf("ios real: %w", err))
	} else {
		for _, dev := range iosDeviceEntries {
			allDevices = append(allDevices, IOSDevice{
				Udid:           dev.Udid,
				ProductName:    dev.ProductName,
				ProductVersion: dev.ProductVersion,
				ProductType:    dev.ProductType,
			})
		}
	}

	sims, err := GetBootedSimulators()
	if err != nil {
		log.Printf("Warning: Failed to get iOS simulators: %v", err)
		errs = append(errs, fmt.Errorf("ios simulator: %w", err))
	} else {
		for _, sim := range sims {
			allDevices = append(allDevices, SimulatorDevice{
				Simulator: sim,
			})
		}
	}

	if len(errs) > 0 {
		var errorMessages []string
		for _, e := range errs {
			errorMessages = append(errorMessages, e.Error())
		}
		return allDevices, fmt.Errorf("encountered errors while listing devices: %s", strings.Join(errorMessages, "; "))
	}

	return allDevices, nil
}
