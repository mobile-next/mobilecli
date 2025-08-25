package devices

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices/ios"
	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/utils"
)

var (
	portForwarder *ios.PortForwarder
)

type IOSDevice struct {
	Udid       string `json:"UniqueDeviceID"`
	DeviceName string `json:"DeviceName"`

	tunnelManager *ios.TunnelManager
	wdaClient     *wda.WdaClient
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
	cmdName, err := ios.FindGoIosPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find go-ios path: %w", err)
	}

	cmd := exec.Command(cmdName, args...)
	output, err := cmd.Output()
	return output, err
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

	device.tunnelManager = ios.NewTunnelManager(udid)
	device.wdaClient = wda.NewWdaClient("localhost:8100")
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
	return d.wdaClient.TakeScreenshot()
}

func (d IOSDevice) Reboot() error {
	_, err := runGoIosCommand("reboot", "--udid", d.ID())
	return err
}

func (d IOSDevice) Tap(x, y int) error {
	return d.wdaClient.Tap(x, y)
}

func (d IOSDevice) Gesture(actions []wda.TapAction) error {
	return d.wdaClient.Gesture(actions)
}

type Tunnel struct {
	Address          string `json:"address"`
	RsdPort          int    `json:"rsdPort"`
	UDID             string `json:"udid"`
	UserspaceTun     bool   `json:"userspaceTun"`
	UserspaceTunPort int    `json:"userspaceTunPort"`
}

func (d IOSDevice) ListTunnels() ([]Tunnel, error) {
	output, err := runGoIosCommand("tunnel", "ls", "--udid", d.ID())
	if err != nil {
		// if no tunnels found, go-ios might return err 1
		return []Tunnel{}, nil
	}

	var tunnels []Tunnel
	err = json.Unmarshal(output, &tunnels)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tunnel list: %w", err)
	}

	return tunnels, nil
}

func (d *IOSDevice) StartTunnel() error {
	return d.tunnelManager.StartTunnel()
}

func (d *IOSDevice) StartTunnelWithCallback(onProcessDied func(error)) error {
	return d.tunnelManager.StartTunnelWithCallback(onProcessDied)
}

func (d *IOSDevice) StopTunnel() error {
	return d.tunnelManager.StopTunnel()
}

func (d *IOSDevice) GetTunnelPID() int {
	return d.tunnelManager.GetTunnelPID()
}

func (d *IOSDevice) StartAgent() error {

	// starting an agent on a real device requires quite a few things to happen in the right order:
	// 1. we check if agent is installed on device (with custom bundle identifier). if we don't have it, this is the process:
	//    a. we download the wda bundle from github
	//    b. we need to unzip it to a temp directory
	//    c. we need to modify the Info.plist to set the correct bundle identifier
	//    d. we need to create an entitlements file
	//    e. we need to sign the bundle
	//    f. we need to install the bundle to the device
	// 2. we need to launch the agent ✅
	// 3. we need to make sure there's a tunnel running for iOS17+
	// 4. we need to set up a forward proxy to port 8100 on the device
	// 5. we need to wait for the agent  to be ready

	_, err := d.wdaClient.GetStatus()
	if err != nil {
		utils.Verbose("WebdriverAgent is not running, starting it")

		// list apps on device
		apps, err := d.ListApps()
		if err != nil {
			return fmt.Errorf("failed to list apps: %w", err)
		}

		// check if WebDriverAgent is installed
		webdriverBundleId := ""
		for _, app := range apps {
			if app.AppName == "WebDriverAgentRunner-Runner" {
				utils.Verbose("WebDriverAgent is installed, launching it")
				webdriverBundleId = app.PackageName
				break
			}
		}

		if webdriverBundleId == "" {
			return fmt.Errorf("WebDriverAgent is not installed")
		}

		// check if tunnel is running
		tunnels, err := d.ListTunnels()
		if err != nil {
			return fmt.Errorf("failed to list tunnels: %w", err)
		}

		if len(tunnels) > 0 {
			utils.Verbose("Tunnels available for this device: %v", tunnels)
		}

		if len(tunnels) == 0 {
			utils.Verbose("No tunnels found, starting a new tunnel")
			err = d.StartTunnel()
			if err != nil {
				return fmt.Errorf("failed to start tunnel: %w", err)
			}

			time.Sleep(1 * time.Second)
		}

		// check that forward proxy is running
		port, err := findAvailablePort()
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}

		portForwarder = ios.NewPortForwarder(d.ID())
		err = portForwarder.Forward(port, 8100)
		if err != nil {
			return fmt.Errorf("failed to forward port: %w", err)
		}

		d.wdaClient = wda.NewWdaClient(fmt.Sprintf("http://localhost:%d", port))

		// check if wda is already running, now that we have a port forwarder set up
		_, err = d.wdaClient.GetStatus()
		if err == nil {
			utils.Verbose("WebDriverAgent is already running")
		}

		if err != nil {
			// launch WebDriverAgent
			utils.Verbose("Launching WebDriverAgent")
			err = d.LaunchApp(webdriverBundleId)
			if err != nil {
				return fmt.Errorf("failed to launch WebDriverAgent: %w", err)
			}

			// wait for WebDriverAgent to start
			utils.Verbose("Waiting for WebDriverAgent to start")
			err = d.wdaClient.WaitForAgent()
			if err != nil {
				return fmt.Errorf("failed to wait for WebDriverAgent: %w", err)
			}

			// wait 1 second after pressing home, so we make sure wda is in the background
			d.wdaClient.PressButton("HOME")
			time.Sleep(1 * time.Second)

			utils.Verbose("WebDriverAgent started")
		}

		return nil
	}

	return err
}

func (d IOSDevice) PressButton(key string) error {
	return d.wdaClient.PressButton(key)
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
	return d.wdaClient.SendKeys(text)
}

func (d IOSDevice) OpenURL(url string) error {
	return d.wdaClient.OpenURL(url)
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
	wdaSize, err := d.wdaClient.GetWindowSize()
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
	return d.wdaClient.StartScreenCapture(format, callback)
}

func findAvailablePort() (int, error) {
	for port := 8100; port <= 8199; port++ {
		if utils.IsPortAvailable("localhost", port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found in range 8101-8199")
}
