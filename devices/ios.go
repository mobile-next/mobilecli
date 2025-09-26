package devices

import (
	"context"
	"fmt"
	"io"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/diagnostics"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/testmanagerd"
	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"github.com/mobile-next/mobilecli/devices/ios"
	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/devices/wda/mjpeg"
	"github.com/mobile-next/mobilecli/utils"
	log "github.com/sirupsen/logrus"
)

var (
	portForwarder *ios.PortForwarder
)

type IOSDevice struct {
	Udid       string `json:"UniqueDeviceID"`
	DeviceName string `json:"DeviceName"`
	OSVersion  string `json:"Version"`

	tunnelManager *ios.TunnelManager
	wdaClient     *wda.WdaClient
	mjpegClient   *mjpeg.WdaMjpegClient
}

func (d IOSDevice) ID() string {
	return d.Udid
}

func (d IOSDevice) Name() string {
	return d.DeviceName
}

func (d IOSDevice) Version() string {
	return d.OSVersion
}

func (d IOSDevice) Platform() string {
	return "ios"
}

func (d IOSDevice) DeviceType() string {
	return "real"
}

func getDeviceInfo(deviceEntry goios.DeviceEntry) (IOSDevice, error) {
	log.SetLevel(log.WarnLevel)

	udid := deviceEntry.Properties.SerialNumber

	allValues, err := goios.GetValues(deviceEntry)
	if err != nil {
		return IOSDevice{}, fmt.Errorf("failed getting values for device %s: %w", udid, err)
	}

	device := IOSDevice{
		Udid:       udid,
		DeviceName: allValues.Value.DeviceName,
		OSVersion:  allValues.Value.ProductVersion,
	}

	tunnelManager, err := ios.NewTunnelManager(udid)
	if err != nil {
		return IOSDevice{}, fmt.Errorf("failed to create tunnel manager for device %s: %w", udid, err)
	}
	device.tunnelManager = tunnelManager
	device.wdaClient = wda.NewWdaClient("localhost:8100")
	return device, nil
}

func ListIOSDevices() ([]IOSDevice, error) {
	log.SetLevel(log.WarnLevel)

	deviceList, err := goios.ListDevices()
	if err != nil {
		return []IOSDevice{}, fmt.Errorf("failed getting device list: %w", err)
	}

	devices := make([]IOSDevice, len(deviceList.DeviceList))
	for i, deviceEntry := range deviceList.DeviceList {
		device, err := getDeviceInfo(deviceEntry)
		if err != nil {
			return []IOSDevice{}, fmt.Errorf("failed to get device info: %w", err)
		}
		devices[i] = device
	}

	return devices, nil
}

func (d IOSDevice) TakeScreenshot() ([]byte, error) {
	return d.wdaClient.TakeScreenshot()
}

func (d IOSDevice) Reboot() error {
	log.SetLevel(log.WarnLevel)

	device, err := d.getEnhancedDevice()
	if err != nil {
		return fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	err = diagnostics.Reboot(device)
	if err != nil {
		return fmt.Errorf("reboot failed: %w", err)
	}

	utils.Verbose("Device %s rebooted successfully", d.Udid)
	return nil
}

func (d IOSDevice) Tap(x, y int) error {
	return d.wdaClient.Tap(x, y)
}

func (d IOSDevice) LongPress(x, y int) error {
	return d.wdaClient.LongPress(x, y)
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
	log.SetLevel(log.WarnLevel)

	if d.tunnelManager == nil {
		return nil, fmt.Errorf("tunnel manager not initialized")
	}

	// Use the library-based tunnel manager to get tunnels directly
	tunnelMgr := d.tunnelManager.GetTunnelManager()
	tunnels, err := tunnelMgr.ListTunnels()
	if err != nil {
		// ListTunnels only errors on serious internal problems, not "no tunnels"
		return nil, fmt.Errorf("failed to list tunnels: %w", err)
	}

	var result []Tunnel
	for _, t := range tunnels {
		// Only return tunnels for this device
		if t.Udid == d.Udid {
			result = append(result, Tunnel{
				Address:          t.Address,
				RsdPort:          t.RsdPort,
				UDID:             t.Udid,
				UserspaceTun:     t.UserspaceTUN,
				UserspaceTunPort: t.UserspaceTUNPort,
			})
		}
	}

	return result, nil
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
	// 3. we need to make sure there's a tunnel running for iOS17+ ✅
	// 4. we need to set up a forward proxy to port 8100 on the device ✅
	// 5. we need to set up a forward proxy to port 9100 on the device for MJPEG screencapture
	// 6. we need to wait for the agent to be ready ✅
	// 7. just in case, click HOME button ✅

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
			// launch WebDriverAgent using testmanagerd
			utils.Verbose("Launching WebDriverAgent")
			err = d.LaunchWda(webdriverBundleId, webdriverBundleId, "WebDriverAgentRunner.xctest")
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
			_ = d.wdaClient.PressButton("HOME")
			time.Sleep(1 * time.Second)

			utils.Verbose("WebDriverAgent started")
		}
	}

	// assuming everything went well if we reached this point
	if true {
		portMjpeg, err := findAvailablePort()
		if err != nil {
			return fmt.Errorf("failed to find available port for mjpeg: %w", err)
		}

		portForwarderMjpeg := ios.NewPortForwarder(d.ID())
		err = portForwarderMjpeg.Forward(portMjpeg, 9100)
		if err != nil {
			return fmt.Errorf("failed to forward port for mjpeg: %w", err)
		}

		mjpegUrl := fmt.Sprintf("http://localhost:%d/", portMjpeg)
		d.mjpegClient = mjpeg.NewWdaMjpegClient(mjpegUrl)
		utils.Verbose("Mjpeg client set up on %s", mjpegUrl)
	}

	return nil
}

func (d IOSDevice) LaunchWda(bundleID, testRunnerBundleID, xctestConfig string) error {
	if bundleID == "" && testRunnerBundleID == "" && xctestConfig == "" {
		utils.Verbose("No bundle ids specified, falling back to defaults")
		bundleID, testRunnerBundleID, xctestConfig = "com.facebook.WebDriverAgentRunner.xctrunner", "com.facebook.WebDriverAgentRunner.xctrunner", "WebDriverAgentRunner.xctest"
	}
	
	utils.Verbose("Running wda with bundleid: %s, testbundleid: %s, xctestconfig: %s", bundleID, testRunnerBundleID, xctestConfig)

	device, err := d.getEnhancedDevice()
	if err != nil {
		return fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// start WDA in background using testmanagerd similar to go-ios runwda command
	go func() {
		defer cancel()
		_, err := testmanagerd.RunTestWithConfig(ctx, testmanagerd.TestConfig{
			BundleId:           bundleID,
			TestRunnerBundleId: testRunnerBundleID,
			XctestConfigName:   xctestConfig,
			Env:                map[string]any{},
			Args:               []string{},
			Device:             device,
			Listener:           testmanagerd.NewTestListener(io.Discard, io.Discard, "/tmp"),
		})

		if err != nil {
			utils.Verbose("WebDriverAgent process ended with error: %v", err)
		}
	}()

	utils.Verbose("WebDriverAgent launched in background")
	return nil
}

func (d IOSDevice) PressButton(key string) error {
	return d.wdaClient.PressButton(key)
}

func deviceWithRsdProvider(device goios.DeviceEntry, udid string, address string, rsdPort int) (goios.DeviceEntry, error) {
	rsdService, err := goios.NewWithAddrPortDevice(address, rsdPort, device)
	if err != nil {
		return goios.DeviceEntry{}, fmt.Errorf("could not connect to RSD: %w", err)
	}
	defer func() { _ = rsdService.Close() }()

	rsdProvider, err := rsdService.Handshake()
	if err != nil {
		return goios.DeviceEntry{}, fmt.Errorf("RSD handshake failed: %w", err)
	}

	device1, err := goios.GetDeviceWithAddress(udid, address, rsdProvider)
	if err != nil {
		return goios.DeviceEntry{}, fmt.Errorf("error getting device with address: %w", err)
	}

	device1.UserspaceTUN = device.UserspaceTUN
	device1.UserspaceTUNHost = device.UserspaceTUNHost
	device1.UserspaceTUNPort = device.UserspaceTUNPort

	return device1, nil
}

// getEnhancedDevice gets device info enhanced with tunnel/RSD information for iOS 17+
func (d IOSDevice) getEnhancedDevice() (goios.DeviceEntry, error) {
	const userspaceTunnelHost = "localhost"

	device, err := goios.GetDevice(d.Udid)
	if err != nil {
		return goios.DeviceEntry{}, fmt.Errorf("device not found: %s: %w", d.Udid, err)
	}

	// Get tunnel info directly from our tunnel manager first
	tunnelMgr := d.tunnelManager.GetTunnelManager()
	tunnelInfo, err := tunnelMgr.FindTunnel(d.Udid)
	if err == nil && tunnelInfo.Udid != "" {
		// We have tunnel info from our tunnel manager
		device.UserspaceTUNPort = tunnelInfo.UserspaceTUNPort
		device.UserspaceTUNHost = userspaceTunnelHost
		device.UserspaceTUN = tunnelInfo.UserspaceTUN
		device, err = deviceWithRsdProvider(device, d.Udid, tunnelInfo.Address, tunnelInfo.RsdPort)
		if err != nil {
			utils.Verbose("failed to get device with RSD provider: %v", err)
		}
	} else {
		// Fallback to HTTP API if our tunnel manager doesn't have info
		utils.Verbose("No tunnel info from local tunnel manager, trying HTTP API")
		info, err := tunnel.TunnelInfoForDevice(device.Properties.SerialNumber, "localhost", 60105)
		if err == nil {
			device.UserspaceTUNPort = info.UserspaceTUNPort
			device.UserspaceTUNHost = userspaceTunnelHost
			device.UserspaceTUN = info.UserspaceTUN
			device, err = deviceWithRsdProvider(device, d.Udid, info.Address, info.RsdPort)
			if err != nil {
				utils.Verbose("failed to get device with RSD provider: %v", err)
			}
		} else {
			utils.Verbose("failed to get tunnel info for device %s: %v", d.Udid, err)
			// If both fail, we'll just use the basic device info
			// This will likely fail for iOS 17+ devices that require tunnels
		}
	}

	return device, nil
}

func (d IOSDevice) LaunchApp(bundleID string) error {
	if bundleID == "" {
		return fmt.Errorf("bundleID cannot be empty")
	}

	log.SetLevel(log.WarnLevel)

	device, err := d.getEnhancedDevice()
	if err != nil {
		return fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	pControl, err := instruments.NewProcessControl(device)
	if err != nil {
		return fmt.Errorf("processcontrol failed: %w", err)
	}
	defer func() { _ = pControl.Close() }()

	opts := map[string]any{}
	args := []interface{}{}
	envs := map[string]any{}

	pid, err := pControl.LaunchAppWithArgs(bundleID, args, envs, opts)
	if err != nil {
		return fmt.Errorf("launch app command failed: %w", err)
	}

	utils.Verbose("Process launched with PID: %d", pid)
	return nil
}

func (d IOSDevice) TerminateApp(bundleID string) error {
	if bundleID == "" {
		return fmt.Errorf("bundleID cannot be empty")
	}

	log.SetLevel(log.WarnLevel)

	device, err := d.getEnhancedDevice()
	if err != nil {
		return fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	pControl, err := instruments.NewProcessControl(device)
	if err != nil {
		return fmt.Errorf("processcontrol failed: %w", err)
	}
	defer func() { _ = pControl.Close() }()

	svc, err := installationproxy.New(device)
	if err != nil {
		return fmt.Errorf("installationproxy failed: %w", err)
	}
	defer func() { svc.Close() }()

	response, err := svc.BrowseAllApps()
	if err != nil {
		return fmt.Errorf("browsing apps failed: %w", err)
	}

	var processName string
	for _, app := range response {
		if app.CFBundleIdentifier() == bundleID {
			processName = app.CFBundleExecutable()
			break
		}
	}
	if processName == "" {
		return fmt.Errorf("%s not installed", bundleID)
	}

	service, err := instruments.NewDeviceInfoService(device)
	if err != nil {
		return fmt.Errorf("failed opening deviceInfoService for getting process list: %w", err)
	}
	defer func() { service.Close() }()

	processList, err := service.ProcessList()
	if err != nil {
		return fmt.Errorf("failed to get process list: %w", err)
	}

	for _, p := range processList {
		if p.Name == processName {
			err = pControl.KillProcess(p.Pid)
			if err != nil {
				return fmt.Errorf("kill process failed: %w", err)
			}
			utils.Verbose("%s killed, Pid: %d", bundleID, p.Pid)
			return nil
		}
	}

	return fmt.Errorf("process of %s not found", bundleID)
}

func (d IOSDevice) SendKeys(text string) error {
	return d.wdaClient.SendKeys(text)
}

func (d IOSDevice) OpenURL(url string) error {
	return d.wdaClient.OpenURL(url)
}

func (d IOSDevice) ListApps() ([]InstalledAppInfo, error) {
	log.SetLevel(log.WarnLevel)

	device, err := d.getEnhancedDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	svc, err := installationproxy.New(device)
	if err != nil {
		return nil, fmt.Errorf("installationproxy failed: %w", err)
	}
	defer func() { svc.Close() }()

	response, err := svc.BrowseAllApps()
	if err != nil {
		return nil, fmt.Errorf("browsing all apps failed: %w", err)
	}

	var apps []InstalledAppInfo
	for _, app := range response {
		apps = append(apps, InstalledAppInfo{
			PackageName: app.CFBundleIdentifier(),
			AppName:     app.CFBundleName(),
			Version:     app.CFBundleShortVersionString(),
		})
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
			Version:  d.Version(),
		},
		ScreenSize: &ScreenSize{
			Width:  wdaSize.ScreenSize.Width,
			Height: wdaSize.ScreenSize.Height,
			Scale:  wdaSize.Scale,
		},
	}, nil
}

func (d IOSDevice) StartScreenCapture(format string, quality int, scale float64, callback func([]byte) bool) error {
	return d.mjpegClient.StartScreenCapture(format, callback)
}

func findAvailablePort() (int, error) {
	for port := 8100; port <= 8199; port++ {
		if utils.IsPortAvailable("localhost", port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found in range 8101-8199")
}

func (d IOSDevice) DumpSource() ([]ScreenElement, error) {
	return d.wdaClient.GetSourceElements()
}

func (d IOSDevice) InstallApp(path string) error {
	log.SetLevel(log.WarnLevel)

	device, err := d.getEnhancedDevice()
	if err != nil {
		return fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	svc, err := zipconduit.New(device)
	if err != nil {
		return fmt.Errorf("zipconduit failed: %w", err)
	}
	defer func() { _ = svc.Close() }()

	err = svc.SendFile(path)
	if err != nil {
		return fmt.Errorf("failed to install app: %w", err)
	}

	return nil
}

func (d IOSDevice) UninstallApp(packageName string) (*InstalledAppInfo, error) {
	log.SetLevel(log.WarnLevel)

	device, err := d.getEnhancedDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	svc, err := installationproxy.New(device)
	if err != nil {
		return nil, fmt.Errorf("installationproxy failed: %w", err)
	}
	defer func() { svc.Close() }()

	appInfo := &InstalledAppInfo{
		PackageName: packageName,
	}

	err = svc.Uninstall(packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to uninstall app: %w", err)
	}

	return appInfo, nil
}

// GetOrientation gets the current device orientation
func (d IOSDevice) GetOrientation() (string, error) {
	return d.wdaClient.GetOrientation()
}

// SetOrientation sets the device orientation
func (d IOSDevice) SetOrientation(orientation string) error {
	return d.wdaClient.SetOrientation(orientation)
}
