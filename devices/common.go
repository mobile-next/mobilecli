package devices

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/utils"
)

const (
	// Default MJPEG streaming quality (1-100)
	DefaultMJPEGQuality = 80
	// Default MJPEG streaming scale (0.1-1.0)
	DefaultMJPEGScale = 1.0
)

type ControllableDevice interface {
	ID() string
	Name() string
	Platform() string   // e.g., "ios", "android"
	DeviceType() string // e.g., "real", "simulator", "emulator"

	TakeScreenshot() ([]byte, error)
	Reboot() error
	Tap(x, y int) error
	Gesture(actions []wda.TapAction) error
	StartAgent() error
	SendKeys(text string) error
	PressButton(key string) error
	LaunchApp(bundleID string) error
	TerminateApp(bundleID string) error
	OpenURL(url string) error
	ListApps() ([]InstalledAppInfo, error)
	Info() (*FullDeviceInfo, error)
	StartScreenCapture(format string, quality int, scale float64, callback func([]byte) bool) error
}

// Aggregates all known devices (iOS, Android, Simulators)
func GetAllControllableDevices() ([]ControllableDevice, error) {
	var allDevices []ControllableDevice

	// get Android devices
	androidDevices, err := GetAndroidDevices()
	if err != nil {
		utils.Verbose("Warning: Failed to get Android devices: %v", err)
	} else {
		allDevices = append(allDevices, androidDevices...)
	}

	// get iOS real devices
	iosDevices, err := ListIOSDevices()
	if err != nil {
		utils.Verbose("Warning: Failed to get iOS real devices: %v", err)
	} else {
		for _, device := range iosDevices {
			allDevices = append(allDevices, &device)
		}
	}

	// get iOS simulator devices
	sims, err := GetBootedSimulators()
	if err != nil {
		utils.Verbose("Warning: Failed to get iOS simulators: %v", err)
	} else {
		for _, sim := range sims {
			allDevices = append(allDevices, &SimulatorDevice{
				Simulator: sim,
				wdaClient: wda.NewWdaClient("localhost:8100"),
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
