package devices

import (
	"fmt"
	"time"

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

// ScreenCaptureConfig contains configuration for screen capture operations
type ScreenCaptureConfig struct {
	Format     string
	Quality    int
	Scale      float64
	OnProgress func(message string) // optional progress callback
	OnData     func([]byte) bool    // data callback - return false to stop
}

// StartAgentConfig contains configuration for agent startup operations
type StartAgentConfig struct {
	OnProgress func(message string) // optional progress callback
}

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
	Boot() error     // boot simulator/emulator
	Shutdown() error // shutdown simulator/emulator
	Tap(x, y int) error
	LongPress(x, y int) error
	Swipe(x1, y1, x2, y2 int) error
	Gesture(actions []wda.TapAction) error
	StartAgent(config StartAgentConfig) error
	SendKeys(text string) error
	PressButton(key string) error
	LaunchApp(bundleID string) error
	TerminateApp(bundleID string) error
	OpenURL(url string) error
	ListApps() ([]InstalledAppInfo, error)
	InstallApp(path string) error
	UninstallApp(packageName string) (*InstalledAppInfo, error)
	Info() (*FullDeviceInfo, error)
	StartScreenCapture(config ScreenCaptureConfig) error
	DumpSource() ([]ScreenElement, error)
	GetOrientation() (string, error)
	SetOrientation(orientation string) error
}

// GetAllControllableDevices aggregates all known devices with options
func GetAllControllableDevices(includeOffline bool) ([]ControllableDevice, error) {
	var allDevices []ControllableDevice

	startTotal := time.Now()

	// get Android devices
	startAndroid := time.Now()
	androidDevices, err := GetAndroidDevices()
	androidDuration := time.Since(startAndroid).Milliseconds()
	androidCount := 0
	if err != nil {
		utils.Verbose("Warning: Failed to get Android devices: %v", err)
	} else {
		androidCount = len(androidDevices)
		allDevices = append(allDevices, androidDevices...)
	}

	// get offline Android emulators if requested
	offlineAndroidCount := 0
	offlineAndroidDuration := int64(0)
	if includeOffline {
		startOfflineAndroid := time.Now()
		// build map of online device IDs for quick lookup
		onlineDeviceIDs := make(map[string]bool)
		for _, device := range androidDevices {
			onlineDeviceIDs[device.ID()] = true
		}

		offlineEmulators, err := getOfflineAndroidEmulators(onlineDeviceIDs)
		offlineAndroidDuration = time.Since(startOfflineAndroid).Milliseconds()
		if err != nil {
			utils.Verbose("Warning: Failed to get offline Android emulators: %v", err)
		} else {
			offlineAndroidCount = len(offlineEmulators)
			allDevices = append(allDevices, offlineEmulators...)
		}
	}

	// get iOS real devices
	startIOS := time.Now()
	iosDevices, err := ListIOSDevices()
	iosDuration := time.Since(startIOS).Milliseconds()
	iosCount := 0
	if err != nil {
		utils.Verbose("Warning: Failed to get iOS real devices: %v", err)
	} else {
		iosCount = len(iosDevices)
		for i := range iosDevices {
			allDevices = append(allDevices, &iosDevices[i])
		}
	}

	// get iOS simulator devices (all simulators, not just booted ones)
	startSimulators := time.Now()
	sims, err := GetSimulators()
	simulatorsDuration := time.Since(startSimulators).Milliseconds()
	simulatorsCount := 0
	if err != nil {
		utils.Verbose("Warning: Failed to get iOS simulators: %v", err)
	} else {
		// filter to only include simulators that have been booted at least once
		filteredSims := filterSimulatorsByDownloadsDirectory(sims)
		simulatorsCount = len(filteredSims)
		for _, sim := range filteredSims {
			allDevices = append(allDevices, &SimulatorDevice{
				Simulator: sim,
				wdaClient: nil,
			})
		}
	}

	totalDuration := time.Since(startTotal).Milliseconds()

	// log all timing stats in one verbose message
	if false {
		utils.Verbose("GetAllControllableDevices completed in %dms: android=%dms (%d devices), offline_android=%dms (%d devices), ios=%dms (%d devices), simulators=%dms (%d devices)",
			totalDuration, androidDuration, androidCount, offlineAndroidDuration, offlineAndroidCount, iosDuration, iosCount, simulatorsDuration, simulatorsCount)
	}

	return allDevices, nil
}

// DeviceInfo represents the JSON-friendly device information
// DeviceListOptions configures device listing behavior
type DeviceListOptions struct {
	IncludeOffline bool
	Platform       string
	DeviceType     string
}

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
func GetDeviceInfoList(opts DeviceListOptions) ([]DeviceInfo, error) {
	devices, err := GetAllControllableDevices(opts.IncludeOffline)
	if err != nil {
		return nil, fmt.Errorf("error getting devices: %w", err)
	}

	deviceInfoList := make([]DeviceInfo, 0, len(devices))
	for _, d := range devices {
		state := d.State()

		// filter offline devices unless includeOffline is true
		if !opts.IncludeOffline && state == "offline" {
			continue
		}

		// filter by platform if specified
		if opts.Platform != "" && d.Platform() != opts.Platform {
			continue
		}

		// filter by device type if specified
		if opts.DeviceType != "" && d.DeviceType() != opts.DeviceType {
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
