package devices

import (
	"fmt"

	"github.com/mobile-next/mobilecli/utils"
)

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
	OpenURL(url string) error
	ListApps() ([]InstalledAppInfo, error)
	Info() (*FullDeviceInfo, error)
	StartScreenCapture(format string, callback func([]byte) bool) error
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
		utils.Verbose("Warning: Failed to get Android devices: %v", err)
		errs = append(errs, fmt.Errorf("android: %w", err))
	} else {
		allDevices = append(allDevices, androidDevices...)
	}

	// Get iOS real devices
	iosDevices, err := ListIOSDevices()
	if err != nil {
		utils.Verbose("Warning: Failed to get iOS real devices: %v", err)
		errs = append(errs, fmt.Errorf("ios real: %w", err))
	} else {
		for _, device := range iosDevices {
			allDevices = append(allDevices, device)
		}
	}

	sims, err := GetBootedSimulators()
	if err != nil {
		utils.Verbose("Warning: Failed to get iOS simulators: %v", err)
		errs = append(errs, fmt.Errorf("ios simulator: %w", err))
	} else {
		for _, sim := range sims {
			allDevices = append(allDevices, &SimulatorDevice{
				Simulator: sim,
			})
		}
	}

	return allDevices, nil
}

// DeviceInfo represents the JSON-friendly device information
type DeviceInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
}

type ScreenSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Scale  int `json:"scale"`
}

type FullDeviceInfo struct {
	DeviceInfo
	ScreenSize *ScreenSize `json:"screenSize"`
}

// GetDeviceInfoList returns a list of DeviceInfo for all connected devices
func GetDeviceInfoList() ([]DeviceInfo, error) {
	devices, err := GetAllControllableDevices()
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	deviceInfoList := make([]DeviceInfo, len(devices))
	for i, d := range devices {
		deviceInfoList[i] = DeviceInfo{
			ID:       d.ID(),
			Name:     d.Name(),
			Platform: d.Platform(),
			Type:     d.DeviceType(),
		}
	}

	return deviceInfoList, nil
}

// InstalledAppInfo represents information about an installed application.
type InstalledAppInfo struct {
	PackageName string `json:"packageName"`
	AppName     string `json:"appName,omitempty"`
	Version     string `json:"version,omitempty"`
}
