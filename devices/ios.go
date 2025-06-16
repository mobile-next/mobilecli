package devices

import (
	"encoding/json"
	"os/exec"

	"github.com/mobile-next/mobilecli/devices/wda"
)

type IOSDevice struct {
	Udid           string `json:"Udid"`
	ProductName    string `json:"ProductName"`
	ProductVersion string `json:"ProductVersion"`
	ProductType    string `json:"ProductType"`
}

type listDevicesResponse struct {
	Devices []IOSDevice `json:"deviceList"`
}

func (d IOSDevice) ID() string {
	return d.Udid
}

func (d IOSDevice) Name() string {
	return d.ProductName + " " + d.ProductVersion
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

// listIOSDevices returns a list of all connected iOS devices using go-ios library
func ListIOSDevices() ([]IOSDevice, error) {
	output, err := runGoIosCommand("list", "--details")
	if err != nil {
		return []IOSDevice{}, err
	}

	var devices listDevicesResponse
	err = json.Unmarshal(output, &devices)
	if err != nil {
		return []IOSDevice{}, err
	}

	return devices.Devices, nil
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
