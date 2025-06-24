package devices

import (
	"encoding/json"
	"os/exec"

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
	return cmd.Output()
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
