package devices

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

const (
	// Default MJPEG streaming quality (1-100)
	DefaultMJPEGQuality = 80
	// Default MJPEG streaming scale (0.1-1.0)
	DefaultMJPEGScale = 1.0
	// Default MJPEG streaming framerate (frames per second)
	DefaultMJPEGFramerate = 30
)

// ScreenElementRect represents the rectangle coordinates and dimensions
// Re-export types for backward compatibility
type ScreenElementRect = types.ScreenElementRect
type ScreenElement = types.ScreenElement

type ControllableDevice interface {
	ID() string
	Name() string
	Platform() string   // e.g., "ios", "android"
	DeviceType() string // e.g., "real", "simulator", "emulator"
	Version() string    // OS version
	State() string      // e.g., "online", "offline"

	TakeScreenshot() ([]byte, error)
	Reboot() error
	Tap(x, y int) error
	LongPress(x, y int) error
	Swipe(x1, y1, x2, y2 int) error
	Gesture(actions []wda.TapAction) error
	StartAgent() error
	SendKeys(text string) error
	PressButton(key string) error
	LaunchApp(bundleID string) error
	TerminateApp(bundleID string) error
	OpenURL(url string) error
	ListApps() ([]InstalledAppInfo, error)
	InstallApp(path string) error
	UninstallApp(packageName string) (*InstalledAppInfo, error)
	Info() (*FullDeviceInfo, error)
	StartScreenCapture(format string, quality int, scale float64, callback func([]byte) bool) error
	DumpSource() ([]ScreenElement, error)
	GetOrientation() (string, error)
	SetOrientation(orientation string) error
}

// Aggregates all known devices (iOS, Android, Simulators)
func GetAllControllableDevices() ([]ControllableDevice, error) {
	return GetAllControllableDevicesWithOptions(false)
}

// GetAllControllableDevicesWithOptions aggregates all known devices with options
func GetAllControllableDevicesWithOptions(includeOffline bool) ([]ControllableDevice, error) {
	var allDevices []ControllableDevice

	// get Android devices
	androidDevices, err := GetAndroidDevices()
	if err != nil {
		utils.Verbose("Warning: Failed to get Android devices: %v", err)
	} else {
		allDevices = append(allDevices, androidDevices...)
	}

	// get offline Android emulators if requested
	if includeOffline {
		// build map of online device IDs for quick lookup
		onlineDeviceIDs := make(map[string]bool)
		for _, device := range androidDevices {
			onlineDeviceIDs[device.ID()] = true
		}

		offlineEmulators, err := getOfflineAndroidEmulators(onlineDeviceIDs)
		if err != nil {
			utils.Verbose("Warning: Failed to get offline Android emulators: %v", err)
		} else {
			allDevices = append(allDevices, offlineEmulators...)
		}
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

	// get iOS simulator devices (all simulators, not just booted ones)
	sims, err := GetSimulators()
	if err != nil {
		utils.Verbose("Warning: Failed to get iOS simulators: %v", err)
	} else {
		// filter to only include simulators that have been booted at least once
		filteredSims := filterSimulatorsByDownloadsDirectory(sims)
		for _, sim := range filteredSims {
			allDevices = append(allDevices, &SimulatorDevice{
				Simulator: sim,
				wdaClient: nil,
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
	Version  string `json:"version"`
	State    string `json:"state"`
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
func GetDeviceInfoList(showAll bool, platform string, deviceType string) ([]DeviceInfo, error) {
	devices, err := GetAllControllableDevicesWithOptions(showAll)
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %v", err)
	}

	deviceInfoList := make([]DeviceInfo, 0, len(devices))
	for _, d := range devices {
		state := d.State()

		// filter offline devices unless showAll is true
		if !showAll && state == "offline" {
			continue
		}

		// filter by platform if specified
		if platform != "" && d.Platform() != platform {
			continue
		}

		// filter by device type if specified
		if deviceType != "" && d.DeviceType() != deviceType {
			continue
		}

		deviceInfoList = append(deviceInfoList, DeviceInfo{
			ID:       d.ID(),
			Name:     d.Name(),
			Platform: d.Platform(),
			Type:     d.DeviceType(),
			Version:  d.Version(),
			State:    state,
		})
	}

	return deviceInfoList, nil
}

// InstalledAppInfo represents information about an installed application.
type InstalledAppInfo struct {
	PackageName string `json:"packageName"`
	AppName     string `json:"appName,omitempty"`
	Version     string `json:"version,omitempty"`
}
