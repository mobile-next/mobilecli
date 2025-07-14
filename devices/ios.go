package devices

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mobile-next/mobilecli/devices/wda"
)

type IOSDevice struct {
	Udid       string `json:"UniqueDeviceID"`
	DeviceName string `json:"DeviceName"`
}

type listDevicesResponse struct {
	Devices []string `json:"deviceList"`
}

func (d IOSDevice) ID() string {
	return d.Udid
}

func (d IOSDevice) Name() string {
	return d.DeviceName
}

func (d IOSDevice) Platform() string {
	return "ios"
}

func (d IOSDevice) DeviceType() string {
	return "real"
}

func runGoIosCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("go-ios", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go-ios command failed: %w", err)
	}
	return output, nil
}

func getDeviceInfo(udid string) (IOSDevice, error) {
	output, err := runGoIosCommand("info", "--udid", udid)
	if err != nil {
		return IOSDevice{}, err
	}

	var device IOSDevice
	err = json.Unmarshal(output, &device)
	if err != nil {
		return IOSDevice{}, err
	}

	return device, nil
}

func ListIOSDevices() ([]IOSDevice, error) {
	output, err := runGoIosCommand("list")
	if err != nil {
		return []IOSDevice{}, err
	}

	var response listDevicesResponse
	err = json.Unmarshal(output, &response)
	if err != nil {
		return []IOSDevice{}, err
	}

	devices := make([]IOSDevice, len(response.Devices))
	for i, udid := range response.Devices {
		device, err := getDeviceInfo(udid)
		if err != nil {
			return []IOSDevice{}, err
		}
		devices[i] = device
	}

	return devices, nil
}

func (d IOSDevice) TakeScreenshot() ([]byte, error) {
	return wda.TakeScreenshot()
}

func (d IOSDevice) Reboot() error {
	_, err := runGoIosCommand("reboot", "--udid", d.ID())
	return err
}

func (d IOSDevice) Tap(x, y int) error {
	return wda.Tap(x, y)
}

func (d IOSDevice) Gesture(actions []interface{}) error {
	// Convert []interface{} to []TapAction
	tapActions := make([]wda.TapAction, len(actions))
	for i, action := range actions {
		actionBytes, err := json.Marshal(action)
		if err != nil {
			return fmt.Errorf("failed to marshal action: %v", err)
		}
		
		var tapAction wda.TapAction
		if err := json.Unmarshal(actionBytes, &tapAction); err != nil {
			return fmt.Errorf("failed to unmarshal action: %v", err)
		}
		tapActions[i] = tapAction
	}
	
	return wda.Gesture(tapActions)
}

func (d IOSDevice) StartAgent() error {
	_, err := wda.GetWebDriverAgentStatus()
	return err
}

func (d IOSDevice) PressButton(key string) error {
	return wda.PressButton(key)
}

func (d IOSDevice) LaunchApp(bundleID string) error {
	_, err := runGoIosCommand("launch", bundleID)
	return err
}

func (d IOSDevice) TerminateApp(bundleID string) error {
	_, err := runGoIosCommand("kill", bundleID)
	return err
}

func (d IOSDevice) SendKeys(text string) error {
	return wda.SendKeys(text)
}

func (d IOSDevice) OpenURL(url string) error {
	return wda.OpenURL(url)
}

func (d IOSDevice) ListApps() ([]InstalledAppInfo, error) {
	output, err := runGoIosCommand("apps", "--all", "--list")
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var apps []InstalledAppInfo

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, " ")
		if len(parts) >= 2 {
			packageName := parts[0]
			version := parts[len(parts)-1]
			appName := strings.Join(parts[1:len(parts)-1], " ")

			apps = append(apps, InstalledAppInfo{
				PackageName: packageName,
				AppName:     appName,
				Version:     version,
			})
		}
	}

	return apps, nil
}

func (d IOSDevice) Info() (*FullDeviceInfo, error) {
	wdaSize, err := wda.GetWindowSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get window size from WDA: %w", err)
	}

	return &FullDeviceInfo{
		DeviceInfo: DeviceInfo{
			ID:       d.ID(),
			Name:     d.Name(),
			Platform: d.Platform(),
			Type:     d.DeviceType(),
		},
		ScreenSize: &ScreenSize{
			Width:  wdaSize.ScreenSize.Width,
			Height: wdaSize.ScreenSize.Height,
			Scale:  wdaSize.Scale,
		},
	}, nil
}

func (d IOSDevice) StartScreenCapture(format string, callback func([]byte) bool) error {
	return wda.StartScreenCapture(format, callback)
}
