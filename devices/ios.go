package devices

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/crashreport"
	"github.com/danielpaulus/go-ios/ios/diagnostics"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/testmanagerd"
	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	lru "github.com/hashicorp/golang-lru/v2"
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
	agentRunnerBundleID       = "com.mobilenext.devicekit-iosUITests.xctrunner"
)

// deviceInfoCache caches device name and OS version to avoid expensive GetValues() calls
type deviceInfoCacheEntry struct {
	DeviceName  string
	OSVersion   string
	ProductType string
}

var (
	deviceInfoCache     *lru.Cache[string, deviceInfoCacheEntry]
	deviceInfoCacheOnce sync.Once
)

// getDeviceInfoCache returns the singleton cache instance
func getDeviceInfoCache() *lru.Cache[string, deviceInfoCacheEntry] {
	deviceInfoCacheOnce.Do(func() {
		var err error
		deviceInfoCache, err = lru.New[string, deviceInfoCacheEntry](32)
		if err != nil {
			// should never happen with valid size
			panic(fmt.Sprintf("failed to create device info cache: %v", err))
		}
	})
	return deviceInfoCache
}

type IOSDevice struct {
	Udid        string `json:"UniqueDeviceID"`
	DeviceName  string `json:"DeviceName"`
	OSVersion   string `json:"Version"`
	ProductType string `json:"ProductType"`

	mu                     sync.Mutex // protects fields below
	tunnelManager          *ios.TunnelManager
	wdaClient              *wda.WdaClient
	mjpegClient            *mjpeg.WdaMjpegClient
	wdaCancel              context.CancelFunc
	portForwarderWda       *ios.PortForwarder
	portForwarderMjpeg     *ios.PortForwarder
	portForwarderDeviceKit *ios.PortForwarder // devicekit http forwarder
	portForwarderAvc       *ios.PortForwarder // devicekit h264 stream forwarder
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

	// check cache first
	cache := getDeviceInfoCache()
	var deviceName, osVersion, productType string

	if cached, ok := cache.Get(udid); ok {
		deviceName = cached.DeviceName
		osVersion = cached.OSVersion
		productType = cached.ProductType
	} else {
		allValues, err := goios.GetValues(deviceEntry)
		if err != nil {
			return IOSDevice{}, fmt.Errorf("failed getting values for device %s: %w", udid, err)
		}

		deviceName = allValues.Value.DeviceName
		osVersion = allValues.Value.ProductVersion
		productType = allValues.Value.ProductType

		// store in cache
		cache.Add(udid, deviceInfoCacheEntry{
			DeviceName:  deviceName,
			OSVersion:   osVersion,
			ProductType: productType,
		})
	}

	device := IOSDevice{
		Udid:        udid,
		DeviceName:  deviceName,
		OSVersion:   osVersion,
		ProductType: productType,
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
	if !d.hasResourcesToCleanup() {
		return nil
	}

	utils.Verbose("Starting cleanup for device %s (%s)", d.Udid, d.DeviceName)
	var errs []error

	// cleanup each resource type
	if err := d.cleanupWDA(); err != nil {
		errs = append(errs, err)
	}

	if err := d.cleanupPortForwarders(); err != nil {
		errs = append(errs, err)
	}

	if err := d.cleanupTunnel(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("device cleanup failed with %d error(s): %v", len(errs), errs)
	}

	return nil
}

// hasResourcesToCleanup checks if there are any resources that need cleanup
func (d *IOSDevice) hasResourcesToCleanup() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	hasWda := d.wdaCancel != nil
	hasWdaPort := d.portForwarderWda != nil && d.portForwarderWda.IsRunning()
	hasMjpegPort := d.portForwarderMjpeg != nil && d.portForwarderMjpeg.IsRunning()
	hasHTTPPort := d.portForwarderDeviceKit != nil && d.portForwarderDeviceKit.IsRunning()
	hasStreamPort := d.portForwarderAvc != nil && d.portForwarderAvc.IsRunning()
	hasTunnel := d.tunnelManager != nil && d.tunnelManager.IsTunnelRunning()

	return hasWda || hasWdaPort || hasMjpegPort || hasHTTPPort || hasStreamPort || hasTunnel
}

// cleanupWDA cancels the WebDriverAgent context
func (d *IOSDevice) cleanupWDA() error {
	d.mu.Lock()
	cancel := d.wdaCancel
	d.wdaCancel = nil
	d.mu.Unlock()

	if cancel != nil {
		utils.Verbose("Canceling WebDriverAgent for device %s", d.Udid)
		cancel()
	}

	return nil
}

// cleanupPortForwarders stops WDA, MJPEG, and DeviceKit port forwarders
func (d *IOSDevice) cleanupPortForwarders() error {
	d.mu.Lock()
	wdaForwarder := d.portForwarderWda
	mjpegForwarder := d.portForwarderMjpeg
	httpForwarder := d.portForwarderDeviceKit
	streamForwarder := d.portForwarderAvc
	d.mu.Unlock()

	var errs []error

	if wdaForwarder != nil && wdaForwarder.IsRunning() {
		utils.Verbose("Stopping WDA port forwarder for device %s", d.Udid)
		if err := wdaForwarder.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop WDA port forwarder: %w", err))
		}
	}

	if mjpegForwarder != nil && mjpegForwarder.IsRunning() {
		utils.Verbose("Stopping mjpeg port forwarder for device %s", d.Udid)
		if err := mjpegForwarder.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop mjpeg port forwarder: %w", err))
		}
	}

	if httpForwarder != nil && httpForwarder.IsRunning() {
		utils.Verbose("Stopping DeviceKit HTTP port forwarder for device %s", d.Udid)
		if err := httpForwarder.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop DeviceKit HTTP port forwarder: %w", err))
		}
	}

	if streamForwarder != nil && streamForwarder.IsRunning() {
		utils.Verbose("Stopping DeviceKit AVC stream port forwarder for device %s", d.Udid)
		if err := streamForwarder.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop DeviceKit AVC stream port forwarder: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("port forwarder cleanup errors: %v", errs)
	}

	return nil
}

// cleanupTunnel stops the tunnel manager
func (d *IOSDevice) cleanupTunnel() error {
	d.mu.Lock()
	tunnel := d.tunnelManager
	d.mu.Unlock()

	if tunnel != nil && tunnel.IsTunnelRunning() {
		utils.Verbose("Stopping tunnel manager for device %s", d.Udid)
		if err := tunnel.StopTunnel(); err != nil {
			return fmt.Errorf("failed to stop tunnel: %w", err)
		}
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

func (d *IOSDevice) ensureWdaPortForwarder() error {
	d.mu.Lock()
	needsNew := d.portForwarderWda == nil || !d.portForwarderWda.IsRunning()
	d.mu.Unlock()

	if needsNew {
		port, err := findAvailablePortInRange(portRangeStart, portRangeEnd)
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		forwarder := ios.NewPortForwarder(d.ID())
		if err = forwarder.Forward(port, deviceKitHTTPPort); err != nil {
			return fmt.Errorf("failed to forward port: %w", err)
		}
		d.mu.Lock()
		d.portForwarderWda = forwarder
		d.wdaClient = wda.NewWdaClient(fmt.Sprintf("http://localhost:%d", port))
		d.mu.Unlock()
		utils.Verbose("WDA port forwarder set up on port %d", port)
		return nil
	}

	d.mu.Lock()
	srcPort, _ := d.portForwarderWda.GetPorts()
	if d.wdaClient == nil {
		d.wdaClient = wda.NewWdaClient(fmt.Sprintf("http://localhost:%d", srcPort))
	}
	d.mu.Unlock()
	utils.Verbose("WDA port forwarder already running on port %d", srcPort)
	return nil
}

func (d *IOSDevice) launchAndWaitForWda(bundleID string, config StartAgentConfig) error {
	if config.OnProgress != nil {
		config.OnProgress("Launching agent")
	}
	if err := d.LaunchTestRunner(bundleID, bundleID, "devicekit-iosUITests.xctest"); err != nil {
		return fmt.Errorf("failed to launch agent: %w", err)
	}
	if config.OnProgress != nil {
		config.OnProgress("Waiting for agent to start")
	}
	if err := d.wdaClient.WaitForAgent(); err != nil {
		return fmt.Errorf("failed to wait for agent: %w", err)
	}
	// background the agent if it jumped to foreground
	if activeApp, err := d.wdaClient.GetActiveAppInfo(); err == nil {
		utils.Verbose("Active app: %s (%s)", activeApp.Name, activeApp.BundleID)
		if activeApp.BundleID == bundleID {
			utils.Verbose("agent is active, pressing HOME to background it")
			_ = d.wdaClient.PressButton("HOME")
			time.Sleep(1 * time.Second)
		}
	}
	return nil
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

	if config.Hook != nil {
		hookName := fmt.Sprintf("ios-device-%s", d.Udid)
		config.Hook.Register(hookName, d.Cleanup)
	}

	if _, err := d.wdaClient.GetStatus(); err == nil {
		return nil // already running
	}
	utils.Verbose("WebdriverAgent is not running, starting it")

	apps, err := d.ListApps(true)
	if err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}

	agentBundleId := ""
	for _, app := range apps {
		if app.PackageName == agentRunnerBundleID {
			utils.Verbose("agent is installed, launching it")
			agentBundleId = app.PackageName
			break
		}
	}
	if agentBundleId == "" {
		return fmt.Errorf("agent is not installed, use 'mobilecli agent install --device %s --provisioning-profile <path>' to install it", d.ID())
	}

	if config.OnProgress != nil {
		config.OnProgress("Starting tunnel")
	}
	if err = d.startTunnel(); err != nil {
		return err
	}

	if err = d.ensureWdaPortForwarder(); err != nil {
		return err
	}

	status, err := d.wdaClient.GetStatus()
	utils.Verbose("WebDriverAgent status %s", status)
	if err == nil {
		utils.Verbose("WebDriverAgent is already running")
		return nil
	}

	return d.launchAndWaitForWda(agentBundleId, config)
}

func (d *IOSDevice) LaunchTestRunner(bundleID, testRunnerBundleID, xctestConfig string) error {
	if bundleID == "" && testRunnerBundleID == "" && xctestConfig == "" {
		utils.Verbose("No bundle ids specified, falling back to defaults")
		bundleID, testRunnerBundleID, xctestConfig = agentRunnerBundleID, agentRunnerBundleID, "devicekit-iosUITests.xctest"
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

	// check if wda is already running (thread-safe)
	d.mu.Lock()
	if d.wdaCancel != nil {
		d.mu.Unlock()
		utils.Verbose("WebDriverAgent is already running")
		return nil
	}

	// create context and store cancel function
	ctx, cancel := context.WithCancel(context.Background())
	d.wdaCancel = cancel
	d.mu.Unlock()

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

		// clear cancel function when done (thread-safe)
		d.mu.Lock()
		d.wdaCancel = nil
		d.mu.Unlock()
	}()

	utils.Verbose("WebDriverAgent launched in background")
	return nil
}

func (d *IOSDevice) PressButton(key string) error {
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

func (d IOSDevice) LaunchApp(bundleID string, locales []string) error {
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
	args := []any{}
	envs := map[string]any{}

	if len(locales) > 0 {
		args = append(args, "-AppleLanguages", "("+strings.Join(locales, ", ")+")")
	}

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

func (d *IOSDevice) ListApps(onlyLaunchable bool) ([]InstalledAppInfo, error) {
	log.SetLevel(log.WarnLevel)

	// Lock to prevent concurrent access to usbmuxd (race condition on ReadPair)
	d.mu.Lock()
	defer d.mu.Unlock()

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

func (d *IOSDevice) GetForegroundApp() (*ForegroundAppInfo, error) {
	// get active app info from WDA
	activeApp, err := d.wdaClient.GetActiveAppInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	// get all installed apps to enrich with version information
	apps, err := d.ListApps(true)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	// find the matching app to get full details
	for _, app := range apps {
		if app.PackageName == activeApp.BundleID {
			return &ForegroundAppInfo{
				PackageName: app.PackageName,
				AppName:     app.AppName,
				Version:     app.Version,
			}, nil
		}
	}

	// if app not found in list (e.g., system app), return info from WDA only
	return &ForegroundAppInfo{
		PackageName: activeApp.BundleID,
		AppName:     activeApp.Name,
		Version:     "",
	}, nil
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
			Model:    d.ProductType,
		},
		ScreenSize: &ScreenSize{
			Width:  wdaSize.ScreenSize.Width,
			Height: wdaSize.ScreenSize.Height,
			Scale:  wdaSize.Scale,
		},
	}, nil
}

func (d *IOSDevice) resolveDeviceKitInfo(config ScreenCaptureConfig) (*DeviceKitInfo, error) {
	if !d.isDeviceKitRunning() {
		if config.OnProgress != nil {
			config.OnProgress("Starting DeviceKit for H.264 streaming")
		}
		info, err := d.StartDeviceKit(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to start DeviceKit: %w", err)
		}
		return info, nil
	}

	utils.Verbose("DeviceKit already running, reusing existing session")
	d.mu.Lock()
	hasHTTP := d.portForwarderDeviceKit != nil && d.portForwarderDeviceKit.IsRunning()
	hasStream := d.portForwarderAvc != nil && d.portForwarderAvc.IsRunning()
	d.mu.Unlock()

	if hasHTTP && hasStream {
		d.mu.Lock()
		httpPort, _ := d.portForwarderDeviceKit.GetPorts()
		streamPort, _ := d.portForwarderAvc.GetPorts()
		d.mu.Unlock()
		return &DeviceKitInfo{HTTPPort: httpPort, StreamPort: streamPort}, nil
	}

	info, err := d.ensureDeviceKitPortForwarders()
	if err != nil {
		return nil, fmt.Errorf("failed to create port forwarders: %w", err)
	}
	return info, nil
}

func (d *IOSDevice) startAvcCapture(config ScreenCaptureConfig) error {
	if config.OnProgress != nil {
		config.OnProgress("Checking DeviceKit status")
	}

	deviceKitInfo, err := d.resolveDeviceKitInfo(config)
	if err != nil {
		return err
	}

	if config.OnProgress != nil {
		config.OnProgress(fmt.Sprintf("Connecting to H.264 stream on localhost:%d", deviceKitInfo.StreamPort))
	}

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", deviceKitInfo.StreamPort))
	if err != nil {
		return fmt.Errorf("failed to connect to stream port: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan error, 1)

	go func() {
		defer func() { _ = conn.Close() }()
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
			if n > 0 && !config.OnData(buffer[:n]) {
				done <- nil
				return
			}
		}
	}()

	select {
	case <-sigChan:
		_ = conn.Close()
		utils.Verbose("stream closed by user")
		return nil
	case err := <-done:
		utils.Verbose("stream ended")
		return err
	}
}

func (d *IOSDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	if config.Format == "avc" {
		return d.startAvcCapture(config)
	}

	// mjpeg is served on the same port as the agent HTTP server at /mjpeg
	d.mu.Lock()
	wdaPort, _ := d.portForwarderWda.GetPorts()
	mjpegURL := buildMjpegURL(wdaPort, config.FPS, config.Scale)
	d.mjpegClient = mjpeg.NewWdaMjpegClient(mjpegURL)
	d.mu.Unlock()

	if config.OnProgress != nil {
		config.OnProgress("Starting video stream")
	}

	return d.mjpegClient.StartScreenCapture(config.Format, config.OnData)
}

func (d IOSDevice) DumpSource() ([]ScreenElement, error) {
	return d.wdaClient.GetSourceElements()
}

func (d IOSDevice) DumpSourceRaw() (any, error) {
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
func (d *IOSDevice) waitForElementByName(name string, timeout time.Duration) (*ScreenElement, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for element %q to appear", name)
		case <-ticker.C:
			elements, err := d.DumpSource()
			if err != nil {
				continue
			}
			for i := range elements {
				if elements[i].Name != nil && *elements[i].Name == name {
					return &elements[i], nil
				}
			}
		}
	}
}

func tapCenter(d *IOSDevice, el *ScreenElement) error {
	x := el.Rect.X + el.Rect.Width/2
	y := el.Rect.Y + el.Rect.Height/2
	return d.Tap(x, y)
}

func (d *IOSDevice) clickStartBroadcastButton() error {
	// handle "Press to Start Broadcasting" screen if present
	if firstElements, err := d.DumpSource(); err == nil {
		if hasText(firstElements, "Press to Start Broadcasting") {
			utils.Verbose("Found 'Press to Start Broadcasting' screen; tapping the only button.")
			buttons := filterButtons(firstElements)
			if len(buttons) != 1 {
				return fmt.Errorf("expected exactly one button on 'Press to Start Broadcasting' screen, found %d", len(buttons))
			}
			if err = tapCenter(d, &buttons[0]); err != nil {
				return fmt.Errorf("failed to tap broadcast button: %w", err)
			}
		}
	}

	utils.Verbose("Waiting for BroadcastUploadExtension button to appear...")
	extBtn, err := d.waitForElementByName("BroadcastUploadExtension", 10*time.Second)
	if err != nil {
		return err
	}
	utils.Verbose("Tapping BroadcastUploadExtension button")
	if err = tapCenter(d, extBtn); err != nil {
		return fmt.Errorf("failed to tap BroadcastUploadExtension button: %w", err)
	}

	utils.Verbose("Waiting for Start Broadcast button to appear...")
	startBtn, err := d.waitForElementByName("Start Broadcast", 10*time.Second)
	if err != nil {
		return err
	}
	utils.Verbose("Tapping Start Broadcast button")
	if err = tapCenter(d, startBtn); err != nil {
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

func (d *IOSDevice) ensureDeviceKitPortForwarders() (*DeviceKitInfo, error) {
	var httpPort, streamPort int
	var err error

	// check if HTTP forwarder exists, create if needed
	d.mu.Lock()
	hasHTTPForwarder := d.portForwarderDeviceKit != nil && d.portForwarderDeviceKit.IsRunning()
	d.mu.Unlock()

	if !hasHTTPForwarder {
		httpPort, err = findAvailablePortInRange(portRangeStart, portRangeEnd)
		if err != nil {
			return nil, fmt.Errorf("failed to find available port for HTTP: %w", err)
		}

		forwarder := ios.NewPortForwarder(d.ID())
		err = forwarder.Forward(httpPort, deviceKitHTTPPort)
		if err != nil {
			return nil, fmt.Errorf("failed to forward HTTP port: %w", err)
		}

		d.mu.Lock()
		d.portForwarderDeviceKit = forwarder
		d.mu.Unlock()
		utils.Verbose("Port forwarding created: localhost:%d -> device:%d (HTTP)", httpPort, deviceKitHTTPPort)
	} else {
		d.mu.Lock()
		httpPort, _ = d.portForwarderDeviceKit.GetPorts()
		d.mu.Unlock()
	}

	// check if stream forwarder exists, create if needed
	d.mu.Lock()
	hasStreamForwarder := d.portForwarderAvc != nil && d.portForwarderAvc.IsRunning()
	d.mu.Unlock()

	if !hasStreamForwarder {
		streamPort, err = findAvailablePortInRange(portRangeStart, portRangeEnd)
		if err != nil {
			if !hasHTTPForwarder {
				_ = d.portForwarderDeviceKit.Stop()
			}
			return nil, fmt.Errorf("failed to find available port for stream: %w", err)
		}

		d.mu.Lock()
		d.portForwarderAvc = ios.NewPortForwarder(d.ID())
		d.mu.Unlock()

		err = d.portForwarderAvc.Forward(streamPort, deviceKitStreamPort)
		if err != nil {
			if !hasHTTPForwarder {
				_ = d.portForwarderDeviceKit.Stop()
			}
			return nil, fmt.Errorf("failed to forward stream port: %w", err)
		}
		utils.Verbose("Port forwarding created: localhost:%d -> device:%d (H.264 stream)", streamPort, deviceKitStreamPort)
	} else {
		d.mu.Lock()
		streamPort, _ = d.portForwarderAvc.GetPorts()
		d.mu.Unlock()
	}

	return &DeviceKitInfo{
		HTTPPort:   httpPort,
		StreamPort: streamPort,
	}, nil
}

func (d *IOSDevice) isDeviceKitRunning() bool {
	// check if we already have port forwarders running
	d.mu.Lock()
	hasHTTPForwarder := d.portForwarderDeviceKit != nil && d.portForwarderDeviceKit.IsRunning()
	hasStreamForwarder := d.portForwarderAvc != nil && d.portForwarderAvc.IsRunning()
	d.mu.Unlock()

	// if both forwarders exist, DeviceKit is definitely running from our perspective
	if hasHTTPForwarder && hasStreamForwarder {
		utils.Verbose("DeviceKit port forwarders already running")
		return true
	}

	// find an available local port for testing
	testPort, err := findAvailablePortInRange(portRangeStart, portRangeEnd)
	if err != nil {
		utils.Verbose("Could not find available port for DeviceKit check: %v", err)
		return false
	}

	// create temporary port forwarder to device port 12005 (stream)
	testForwarder := ios.NewPortForwarder(d.ID())
	err = testForwarder.Forward(testPort, deviceKitStreamPort)
	if err != nil {
		utils.Verbose("Could not create test port forwarder: %v", err)
		return false
	}

	// ensure cleanup of test forwarder
	defer func() {
		_ = testForwarder.Stop()
	}()

	// try to connect with timeout
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", testPort), 2*time.Second)
	if err != nil {
		utils.Verbose("DeviceKit not responding on port %d: %v", deviceKitStreamPort, err)
		return false
	}
	defer func() { _ = conn.Close() }()

	// set read deadline and try to read 1 byte
	err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		utils.Verbose("Could not set read deadline: %v", err)
		return false
	}

	buffer := make([]byte, 1)
	_, err = conn.Read(buffer)
	if err != nil {
		utils.Verbose("DeviceKit not serving data on port %d: %v", deviceKitStreamPort, err)
		return false
	}

	utils.Verbose("DeviceKit is already running on device port %d", deviceKitStreamPort)
	return true
}

func findDeviceKitBundleID(apps []InstalledAppInfo) (string, error) {
	for _, app := range apps {
		pkg := app.PackageName
		if strings.HasPrefix(pkg, "com.") && strings.Contains(pkg, "devicekit-ios") && !strings.Contains(pkg, "UITests") {
			utils.Verbose("DeviceKit main app found, bundle ID: %s", pkg)
			return pkg, nil
		}
	}
	return "", fmt.Errorf("DeviceKit main app not found. Please install devicekit-ios on the device")
}

func (d *IOSDevice) setupDeviceKitForwarders() (httpPort, streamPort int, err error) {
	httpPort, err = findAvailablePortInRange(portRangeStart, portRangeEnd)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find available port for HTTP: %w", err)
	}
	d.mu.Lock()
	d.portForwarderDeviceKit = ios.NewPortForwarder(d.ID())
	d.mu.Unlock()
	if err = d.portForwarderDeviceKit.Forward(httpPort, deviceKitHTTPPort); err != nil {
		return 0, 0, fmt.Errorf("failed to forward HTTP port: %w", err)
	}
	utils.Verbose("Port forwarding started: localhost:%d -> device:%d (HTTP)", httpPort, deviceKitHTTPPort)

	streamPort, err = findAvailablePortInRange(portRangeStart, portRangeEnd)
	if err != nil {
		_ = d.portForwarderDeviceKit.Stop()
		return 0, 0, fmt.Errorf("failed to find available port for stream: %w", err)
	}
	d.mu.Lock()
	d.portForwarderAvc = ios.NewPortForwarder(d.ID())
	d.mu.Unlock()
	if err = d.portForwarderAvc.Forward(streamPort, deviceKitStreamPort); err != nil {
		_ = d.portForwarderDeviceKit.Stop()
		return 0, 0, fmt.Errorf("failed to forward stream port: %w", err)
	}
	utils.Verbose("Port forwarding started: localhost:%d -> device:%d (H.264 stream)", streamPort, deviceKitStreamPort)
	return httpPort, streamPort, nil
}

// StartDeviceKit starts the devicekit-ios XCUITest which provides:
// - An HTTP server for tap/dumpUI commands (port 12004)
// - A broadcast extension for H.264 screen streaming (port 12005)
func (d *IOSDevice) StartDeviceKit(hook *ShutdownHook) (*DeviceKitInfo, error) {
	if hook != nil {
		hookName := fmt.Sprintf("ios-devicekit-%s", d.Udid)
		hook.Register(hookName, d.Cleanup)
	}

	if err := d.startTunnel(); err != nil {
		return nil, fmt.Errorf("failed to start tunnel: %w", err)
	}

	utils.Verbose("Broadcast extension not running, starting DeviceKit app...")

	apps, err := d.ListApps(true)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}

	devicekitMainAppBundleId, err := findDeviceKitBundleID(apps)
	if err != nil {
		return nil, err
	}

	localHTTPPort, localStreamPort, err := d.setupDeviceKitForwarders()
	if err != nil {
		return nil, err
	}

	failed := true
	defer func() {
		if failed {
			_ = d.portForwarderDeviceKit.Stop()
			_ = d.portForwarderAvc.Stop()
		}
	}()

	utils.Verbose("Launching DeviceKit app: %s", devicekitMainAppBundleId)
	startTime := time.Now()
	if err = d.LaunchApp(devicekitMainAppBundleId, nil); err != nil {
		return nil, fmt.Errorf("failed to launch DeviceKit app: %w", err)
	}

	utils.Verbose("Waiting for DeviceKit app to be in foreground...")
	if err = d.waitForAppInForeground(devicekitMainAppBundleId, deviceKitAppLaunchTimeout); err != nil {
		return nil, fmt.Errorf("failed to wait for DeviceKit app: %w", err)
	}

	if err = d.StartAgent(StartAgentConfig{OnProgress: func(msg string) { utils.Verbose(msg) }}); err != nil {
		return nil, fmt.Errorf("failed to start agent: %w", err)
	}

	if err = d.clickStartBroadcastButton(); err != nil {
		return nil, fmt.Errorf("failed to click Start Broadcast button: %w", err)
	}

	utils.Verbose("DeviceKit startup benchmark: %.2f seconds (from LaunchApp to Start Broadcasting clicked)", time.Since(startTime).Seconds())

	utils.Verbose("Waiting %v for broadcast TCP server to start...", deviceKitBroadcastTimeout)
	time.Sleep(deviceKitBroadcastTimeout)

	for i := 0; i < 3; i++ {
		if err = d.PressButton("HOME"); err != nil {
			utils.Verbose("Failed to press HOME button (attempt %d): %v", i+1, err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	utils.Verbose("DeviceKit broadcast started successfully")
	failed = false
	return &DeviceKitInfo{HTTPPort: localHTTPPort, StreamPort: localStreamPort}, nil
}

// waitForAppInForeground polls WDA to check if the specified app is in foreground
func (d *IOSDevice) waitForAppInForeground(bundleID string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return fmt.Errorf("timeout waiting for app %s to be in foreground", bundleID)
		case <-ticker.C:
			activeApp, err := d.wdaClient.GetActiveAppInfo()
			if err != nil {
				// continue trying on error
				continue
			}

			if activeApp.BundleID == bundleID {
				utils.Verbose("App %s is now in foreground", bundleID)
				return nil
			}
		}
	}
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

func (d *IOSDevice) ListCrashReports() ([]CrashReport, error) {
	device, err := d.getEnhancedDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	files, err := crashreport.ListReports(device, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list crash reports: %w", err)
	}

	return ParseCrashReports(files), nil
}

func (d *IOSDevice) GetCrashReport(id string) ([]byte, error) {
	if strings.Contains(id, "/") || strings.Contains(id, "..") {
		return nil, fmt.Errorf("invalid crash id: %s", id)
	}

	device, err := d.getEnhancedDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "mobilecli-crash-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	err = crashreport.DownloadReports(device, id, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to download crash report: %w", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, id))
	if err != nil {
		return nil, fmt.Errorf("crash %s not found: %w", id, err)
	}

	return content, nil
}
