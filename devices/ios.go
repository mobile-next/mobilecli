package devices

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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

const (
	portRangeStart            = 8100
	portRangeEnd              = 8299
	deviceKitHTTPPort         = 12004 // device-side HTTP server port
	deviceKitStreamPort       = 12005 // device-side H.264 TCP stream port
	deviceKitAppLaunchTimeout = 5 * time.Second
	deviceKitBroadcastTimeout = 5 * time.Second
)

type IOSDevice struct {
	Udid       string `json:"UniqueDeviceID"`
	DeviceName string `json:"DeviceName"`
	OSVersion  string `json:"Version"`

	tunnelManager      *ios.TunnelManager
	wdaClient          *wda.WdaClient
	mjpegClient        *mjpeg.WdaMjpegClient
	wdaCancel          context.CancelFunc
	portForwarder      *ios.PortForwarder
	portForwarderMjpeg *ios.PortForwarder
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

func (d IOSDevice) State() string {
	return "online"
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

	// register device for cleanup tracking
	RegisterDevice(&device)

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

	// ensure tunnel is running for iOS 17+
	err := d.startTunnel()
	if err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

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

func (d IOSDevice) Boot() error {
	return fmt.Errorf("boot is not supported for real iOS devices")
}

func (d IOSDevice) Shutdown() error {
	return fmt.Errorf("shutdown is not supported for real iOS devices")
}

func (d IOSDevice) Tap(x, y int) error {
	return d.wdaClient.Tap(x, y)
}

func (d IOSDevice) LongPress(x, y, duration int) error {
	return d.wdaClient.LongPress(x, y, duration)
}

func (d IOSDevice) Swipe(x1, y1, x2, y2 int) error {
	return d.wdaClient.Swipe(x1, y1, x2, y2)
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

func (d *IOSDevice) StartTunnelWithCallback(onProcessDied func(error)) error {
	return d.tunnelManager.StartTunnelWithCallback(onProcessDied)
}

func (d *IOSDevice) stopTunnel() error {
	return d.tunnelManager.StopTunnel()
}

// Cleanup gracefully cleans up all device resources
func (d *IOSDevice) Cleanup() error {
	// check if there's anything to clean up
	hasWda := d.wdaCancel != nil
	hasWdaPort := d.portForwarder != nil && d.portForwarder.IsRunning()
	hasMjpegPort := d.portForwarderMjpeg != nil && d.portForwarderMjpeg.IsRunning()
	hasTunnel := d.tunnelManager != nil && d.tunnelManager.IsTunnelRunning()

	if !hasWda && !hasWdaPort && !hasMjpegPort && !hasTunnel {
		return nil
	}

	utils.Verbose("Starting cleanup for device %s (%s)", d.Udid, d.DeviceName)
	var errs []error

	// cancel WDA context if running
	if hasWda {
		utils.Verbose("Canceling WebDriverAgent for device %s", d.Udid)
		d.wdaCancel()
		d.wdaCancel = nil
	}

	// stop WDA port forwarder
	if hasWdaPort {
		utils.Verbose("Stopping WDA port forwarder for device %s", d.Udid)
		if err := d.portForwarder.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop WDA port forwarder: %w", err))
		}
	}

	// stop mjpeg port forwarder
	if hasMjpegPort {
		utils.Verbose("Stopping mjpeg port forwarder for device %s", d.Udid)
		if err := d.portForwarderMjpeg.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop mjpeg port forwarder: %w", err))
		}
	}

	// stop tunnel manager if running
	if hasTunnel {
		utils.Verbose("Stopping tunnel manager for device %s", d.Udid)
		if err := d.tunnelManager.StopTunnel(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop tunnel: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}

func (d *IOSDevice) requiresTunnel() bool {
	parts := strings.Split(d.OSVersion, ".")
	if len(parts) == 0 {
		return false
	}

	majorVersion, err := strconv.Atoi(parts[0])
	if err != nil {
		utils.Verbose("failed to parse iOS version %s: %v", d.OSVersion, err)
		return false
	}

	return majorVersion >= 17
}

func (d *IOSDevice) waitForTunnelReady() error {
	tunnelMgr := d.tunnelManager.GetTunnelManager()
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for tunnel to be ready for device %s", d.Udid)
		case <-ticker.C:
			tunnelInfo, err := tunnelMgr.FindTunnel(d.Udid)
			if err == nil && tunnelInfo.Udid != "" {
				utils.Verbose("Tunnel ready for device %s", d.Udid)
				return nil
			}
		}
	}
}

func (d *IOSDevice) startTunnel() error {
	if !d.requiresTunnel() {
		return nil
	}

	// start tunnel if not already running
	// TunnelManager.StartTunnel() will return error if already running
	err := d.tunnelManager.StartTunnel()
	if err != nil {
		// check if it's the "already running" error, which is fine

		if errors.Is(err, ios.ErrTunnelAlreadyRunning) {
			utils.Verbose("Tunnel already running for this device")
			return nil
		}
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	utils.Verbose("Started new tunnel for device %s", d.Udid)
	return d.waitForTunnelReady()
}

func (d *IOSDevice) StartAgent(config StartAgentConfig) error {

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
			if config.OnProgress != nil {
				config.OnProgress("Installing WebDriverAgent")
			}
			return fmt.Errorf("WebDriverAgent is not installed")
		}

		if config.OnProgress != nil {
			config.OnProgress("Starting tunnel")
		}

		// start tunnel if needed (only for iOS 17+)
		err = d.startTunnel()
		if err != nil {
			return err
		}

		// check that forward proxy is running
		port, err := findAvailablePortInRange(portRangeStart, portRangeEnd)
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}

		d.portForwarder = ios.NewPortForwarder(d.ID())
		err = d.portForwarder.Forward(port, 8100)
		if err != nil {
			return fmt.Errorf("failed to forward port: %w", err)
		}

		d.wdaClient = wda.NewWdaClient(fmt.Sprintf("http://localhost:%d", port))

		// check if wda is already running, now that we have a port forwarder set up
		status, err := d.wdaClient.GetStatus()
		if err == nil {
			utils.Verbose("WebDriverAgent is already running")
		}

		utils.Verbose("WebDriverAgent status %s", status)

		if err != nil {
			if config.OnProgress != nil {
				config.OnProgress("Launching WebDriverAgent")
			}

			// launch WebDriverAgent using testmanagerd
			err = d.LaunchTestRunner(webdriverBundleId, webdriverBundleId, "WebDriverAgentRunner.xctest")
			if err != nil {
				return fmt.Errorf("failed to launch WebDriverAgent: %w", err)
			}

			if config.OnProgress != nil {
				config.OnProgress("Waiting for agent to start")
			}

			// wait for WebDriverAgent to start
			err = d.wdaClient.WaitForAgent()
			if err != nil {
				return fmt.Errorf("failed to wait for WebDriverAgent: %w", err)
			}

			// wait 1 second after pressing home, so we make sure wda is in the background
			_ = d.wdaClient.PressButton("HOME")
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}

func (d *IOSDevice) LaunchTestRunner(bundleID, testRunnerBundleID, xctestConfig string) error {
	if bundleID == "" && testRunnerBundleID == "" && xctestConfig == "" {
		utils.Verbose("No bundle ids specified, falling back to defaults")
		bundleID, testRunnerBundleID, xctestConfig = "com.facebook.WebDriverAgentRunner.xctrunner", "com.facebook.WebDriverAgentRunner.xctrunner", "WebDriverAgentRunner.xctest"
	}

	utils.Verbose("Running wda with bundleid: %s, testbundleid: %s, xctestconfig: %s", bundleID, testRunnerBundleID, xctestConfig)

	// ensure tunnel is running for iOS 17+
	err := d.startTunnel()
	if err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	device, err := d.getEnhancedDevice()
	if err != nil {
		return fmt.Errorf("failed to get enhanced device connection: %w", err)
	}

	// if wda is already running, don't launch again
	if d.wdaCancel != nil {
		utils.Verbose("WebDriverAgent is already running")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.wdaCancel = cancel

	// start WDA in background using testmanagerd similar to go-ios runwda command
	go func() {
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
		} else {
			utils.Verbose("WebDriverAgent process ended")
		}
		d.wdaCancel = nil
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

	// ensure tunnel is running for iOS 17+
	err := d.startTunnel()
	if err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

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

	// ensure tunnel is running for iOS 17+
	err := d.startTunnel()
	if err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

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

	// ensure tunnel is running for iOS 17+
	err := d.startTunnel()
	if err != nil {
		return nil, fmt.Errorf("failed to start tunnel: %w", err)
	}

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
			State:    d.State(),
		},
		ScreenSize: &ScreenSize{
			Width:  wdaSize.ScreenSize.Width,
			Height: wdaSize.ScreenSize.Height,
			Scale:  wdaSize.Scale,
		},
	}, nil
}

func (d *IOSDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	// handle avc format via DeviceKit
	if config.Format == "avc" {
		if config.OnProgress != nil {
			config.OnProgress("Starting DeviceKit for H.264 streaming")
		}

		// start DeviceKit
		deviceKitInfo, err := d.StartDeviceKit()
		if err != nil {
			return fmt.Errorf("failed to start DeviceKit: %w", err)
		}

		if config.OnProgress != nil {
			config.OnProgress(fmt.Sprintf("Connecting to H.264 stream on localhost:%d", deviceKitInfo.StreamPort))
		}

		// connect to the TCP stream
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", deviceKitInfo.StreamPort))
		if err != nil {
			return fmt.Errorf("failed to connect to stream port: %w", err)
		}

		// setup signal handling for Ctrl+C
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// channel to signal when streaming is done
		done := make(chan error, 1)

		// stream data in a goroutine
		go func() {
			defer conn.Close()
			buffer := make([]byte, 65536)
			for {
				n, err := conn.Read(buffer)
				if err != nil {
					if err != io.EOF {
						done <- fmt.Errorf("error reading from stream: %w", err)
					} else {
						done <- nil
					}
					return
				}

				if n > 0 {
					if !config.OnData(buffer[:n]) {
						// client wants to stop the stream
						done <- nil
						return
					}
				}
			}
		}()

		// wait for either signal or stream completion
		select {
		case <-sigChan:
			conn.Close()
			utils.Verbose("stream closed by user")
			return nil
		case err := <-done:
			utils.Verbose("stream ended")
			return err
		}
	}

	// handle mjpeg format via WDA
	// set up mjpeg port forwarding if not already running
	if d.portForwarderMjpeg == nil || !d.portForwarderMjpeg.IsRunning() {
		portMjpeg, err := findAvailablePortInRange(portRangeStart, portRangeEnd)
		if err != nil {
			return fmt.Errorf("failed to find available port for mjpeg: %w", err)
		}

		d.portForwarderMjpeg = ios.NewPortForwarder(d.ID())
		err = d.portForwarderMjpeg.Forward(portMjpeg, 9100)
		if err != nil {
			return fmt.Errorf("failed to forward port for mjpeg: %w", err)
		}

		mjpegUrl := fmt.Sprintf("http://localhost:%d/", portMjpeg)
		d.mjpegClient = mjpeg.NewWdaMjpegClient(mjpegUrl)
		utils.Verbose("Mjpeg client set up on %s", mjpegUrl)
	}

	// configure mjpeg framerate
	fps := config.FPS
	if fps == 0 {
		fps = DefaultFramerate
	}
	err := d.wdaClient.SetMjpegFramerate(fps)
	if err != nil {
		return err
	}

	if config.OnProgress != nil {
		config.OnProgress("Starting video stream")
	}

	return d.mjpegClient.StartScreenCapture(config.Format, config.OnData)
}

func (d IOSDevice) DumpSource() ([]ScreenElement, error) {
	return d.wdaClient.GetSourceElements()
}

func (d IOSDevice) DumpSourceRaw() (interface{}, error) {
	return d.wdaClient.GetSourceRaw()
}

func (d IOSDevice) InstallApp(path string) error {
	log.SetLevel(log.WarnLevel)

	// ensure tunnel is running for iOS 17+
	err := d.startTunnel()
	if err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

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

	// ensure tunnel is running for iOS 17+
	err := d.startTunnel()
	if err != nil {
		return nil, fmt.Errorf("failed to start tunnel: %w", err)
	}

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

// DeviceKitInfo contains information about the started DeviceKit session
type DeviceKitInfo struct {
	HTTPPort   int `json:"httpPort"`
	StreamPort int `json:"streamPort"`
}

// clickStartBroadcastButton polls for the "BroadcastUploadExtension" button, taps it,
// then polls for the "Start Broadcast" button and taps it
func (d *IOSDevice) clickStartBroadcastButton() error {
	// first dump: handle "Press to Start Broadcasting" screen if present
	firstElements, err := d.DumpSource()
	if err == nil {
		if hasText(firstElements, "Press to Start Broadcasting") {
			utils.Verbose("Found 'Press to Start Broadcasting' screen; tapping the only button.")
			buttons := filterButtons(firstElements)
			if len(buttons) != 1 {
				return fmt.Errorf("expected exactly one button on 'Press to Start Broadcasting' screen, found %d", len(buttons))
			}

			centerX := buttons[0].Rect.X + buttons[0].Rect.Width/2
			centerY := buttons[0].Rect.Y + buttons[0].Rect.Height/2
			if err = d.Tap(centerX, centerY); err != nil {
				return fmt.Errorf("failed to tap broadcast button: %w", err)
			}
		}
	}

	// first, find and tap "BroadcastUploadExtension"
	utils.Verbose("Waiting for BroadcastUploadExtension button to appear...")
	var broadcastExtensionButton *ScreenElement
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for broadcastExtensionButton == nil {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for BroadcastUploadExtension button to appear")
		case <-ticker.C:
			elements, err := d.DumpSource()
			if err != nil {
				// continue trying on error
				continue
			}

			// find the "BroadcastUploadExtension" button
			for i := range elements {
				if elements[i].Name != nil && *elements[i].Name == "BroadcastUploadExtension" {
					broadcastExtensionButton = &elements[i]
					break
				}
			}
		}
	}

	utils.Verbose("BroadcastUploadExtension button found")

	// calculate center coordinates and tap
	centerX := broadcastExtensionButton.Rect.X + broadcastExtensionButton.Rect.Width/2
	centerY := broadcastExtensionButton.Rect.Y + broadcastExtensionButton.Rect.Height/2
	utils.Verbose("Tapping BroadcastUploadExtension button at (%d, %d)", centerX, centerY)

	err = d.Tap(centerX, centerY)
	if err != nil {
		return fmt.Errorf("failed to tap BroadcastUploadExtension button: %w", err)
	}

	// now wait for "Start Broadcast" button to appear
	utils.Verbose("Waiting for Start Broadcast button to appear...")
	var startBroadcastButton *ScreenElement
	timeout = time.After(10 * time.Second)
	ticker = time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for startBroadcastButton == nil {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for Start Broadcast button to appear")
		case <-ticker.C:
			elements, err := d.DumpSource()
			if err != nil {
				// continue trying on error
				continue
			}

			// find the "Start Broadcast" button
			for i := range elements {
				if elements[i].Name != nil && *elements[i].Name == "Start Broadcast" {
					startBroadcastButton = &elements[i]
					break
				}
			}
		}
	}

	utils.Verbose("Start Broadcast button found")

	// calculate center coordinates and tap
	centerX = startBroadcastButton.Rect.X + startBroadcastButton.Rect.Width/2
	centerY = startBroadcastButton.Rect.Y + startBroadcastButton.Rect.Height/2
	utils.Verbose("Tapping Start Broadcast button at (%d, %d)", centerX, centerY)

	err = d.Tap(centerX, centerY)
	if err != nil {
		return fmt.Errorf("failed to tap Start Broadcast button: %w", err)
	}

	return nil
}

func hasText(elements []ScreenElement, text string) bool {
	for i := range elements {
		if elements[i].Label != nil && *elements[i].Label == text {
			return true
		}
		if elements[i].Name != nil && *elements[i].Name == text {
			return true
		}
		if elements[i].Value != nil && *elements[i].Value == text {
			return true
		}
		if elements[i].Text != nil && *elements[i].Text == text {
			return true
		}
	}
	return false
}

func filterButtons(elements []ScreenElement) []ScreenElement {
	var buttons []ScreenElement
	for i := range elements {
		if elements[i].Type == "Button" {
			buttons = append(buttons, elements[i])
		}
	}
	return buttons
}

// StartDeviceKit starts the devicekit-ios XCUITest which provides:
// - An HTTP server for tap/dumpUI commands (port 12004)
// - A broadcast extension for H.264 screen streaming (port 12005)
func (d *IOSDevice) StartDeviceKit() (*DeviceKitInfo, error) {
	// Start tunnel if needed (iOS 17+)
	err := d.startTunnel()
	if err != nil {
		return nil, fmt.Errorf("failed to start tunnel: %w", err)
	}

	// Broadcast is not running, we need to start it.
	utils.Verbose("Broadcast extension not running, starting DeviceKit app...")

	// find DeviceKit main app (not the xctrunner)
	apps, err := d.ListApps()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	var devicekitMainAppBundleId string
	for _, app := range apps {
		// look for the main app, not the test runner
		if strings.HasPrefix(app.PackageName, "com.") && strings.Contains(app.PackageName, "devicekit-ios") && !strings.Contains(app.PackageName, "UITests") {
			utils.Verbose("DeviceKit main app found, bundle ID: %s", app.PackageName)
			devicekitMainAppBundleId = app.PackageName
			break
		}
	}

	if devicekitMainAppBundleId == "" {
		return nil, fmt.Errorf("DeviceKit main app not found. Please install devicekit-ios on the device")
	}

	// Find available local port for HTTP forwarding and bind immediately.
	localHTTPPort, err := findAvailablePortInRange(portRangeStart, portRangeEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to find available port for HTTP: %w", err)
	}

	httpForwarder := ios.NewPortForwarder(d.ID())
	err = httpForwarder.Forward(localHTTPPort, deviceKitHTTPPort)
	if err != nil {
		return nil, fmt.Errorf("failed to forward HTTP port: %w", err)
	}
	utils.Verbose("Port forwarding started: localhost:%d -> device:%d (HTTP)", localHTTPPort, deviceKitHTTPPort)
	// Find available local port for stream forwarding after HTTP is bound.
	localStreamPort, err := findAvailablePortInRange(portRangeStart, portRangeEnd)
	if err != nil {
		_ = httpForwarder.Stop()
		return nil, fmt.Errorf("failed to find available port for stream: %w", err)
	}

	streamForwarder := ios.NewPortForwarder(d.ID())
	err = streamForwarder.Forward(localStreamPort, deviceKitStreamPort)
	if err != nil {
		// clean up HTTP forwarder on failure
		_ = httpForwarder.Stop()
		return nil, fmt.Errorf("failed to forward stream port: %w", err)
	}
	utils.Verbose("Port forwarding started: localhost:%d -> device:%d (H.264 stream)", localStreamPort, deviceKitStreamPort)

	// Launch the main DeviceKit app
	utils.Verbose("Launching DeviceKit app: %s", devicekitMainAppBundleId)
	err = d.LaunchApp(devicekitMainAppBundleId)
	if err != nil {
		// clean up port forwarders on failure
		_ = httpForwarder.Stop()
		_ = streamForwarder.Stop()
		return nil, fmt.Errorf("failed to launch DeviceKit app: %w", err)
	}

	// Wait for the app to launch and show the broadcast picker
	utils.Verbose("Waiting %v for DeviceKit app to launch...", deviceKitAppLaunchTimeout)
	time.Sleep(deviceKitAppLaunchTimeout)

	// Start WebDriverAgent to be able to tap on the screen
	err = d.StartAgent(StartAgentConfig{
		OnProgress: func(message string) {
			utils.Verbose(message)
		},
	})
	if err != nil {
		// clean up port forwarders on failure
		_ = httpForwarder.Stop()
		_ = streamForwarder.Stop()
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	// find and tap the "Start Broadcast" button
	err = d.clickStartBroadcastButton()
	if err != nil {
		// clean up port forwarders on failure
		_ = httpForwarder.Stop()
		_ = streamForwarder.Stop()
		return nil, fmt.Errorf("failed to click Start Broadcast button: %w", err)
	}

	// Wait for the TCP server to start listening (takes about 5 seconds)
	utils.Verbose("Waiting %v for broadcast TCP server to start...", deviceKitBroadcastTimeout)
	time.Sleep(deviceKitBroadcastTimeout)

	utils.Verbose("DeviceKit broadcast started successfully")

	return &DeviceKitInfo{
		HTTPPort:   localHTTPPort,
		StreamPort: localStreamPort,
	}, nil
}

// findAvailablePortInRange finds an available port in the specified range
func findAvailablePortInRange(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		if utils.IsPortAvailable("localhost", port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found in range %d-%d", start, end)
}
