package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/utils"
)

type coreDeviceListOutput struct {
	Result struct {
		Devices []coreDeviceEntry `json:"devices"`
	} `json:"result"`
}

type coreDeviceEntry struct {
	Identifier           string `json:"identifier"`
	ConnectionProperties struct {
		PairingState    string `json:"pairingState"`
		TransportType   string `json:"transportType"`
		TunnelState     string `json:"tunnelState"`
		TunnelIPAddress string `json:"tunnelIPAddress"`
	} `json:"connectionProperties"`
	DeviceProperties struct {
		BootState       string `json:"bootState"`
		Name            string `json:"name"`
		OSVersionNumber string `json:"osVersionNumber"`
	} `json:"deviceProperties"`
	HardwareProperties struct {
		ProductType string `json:"productType"`
		Reality     string `json:"reality"`
		UDID        string `json:"udid"`
	} `json:"hardwareProperties"`
}

type coreDevicePhysicalInfo struct {
	Identifier  string
	UDID        string
	Name        string
	OSVersion   string
	ProductType string
	Transport   string
	BootState   string
	TunnelState string
	TunnelIP    string
}

type coreDeviceAppsOutput struct {
	Result struct {
		Apps []struct {
			BundleIdentifier string `json:"bundleIdentifier"`
			Name             string `json:"name"`
			Version          string `json:"version"`
		} `json:"apps"`
	} `json:"result"`
}

type coreDeviceDisplaysOutput struct {
	Result struct {
		Displays []struct {
			Bounds     [][]int `json:"bounds"`
			Name       string  `json:"name"`
			Primary    bool    `json:"primary"`
			PointScale int     `json:"pointScale"`
		} `json:"displays"`
	} `json:"result"`
}

type coreDeviceDetails struct {
	Identifier    string
	UDID          string
	Name          string
	OSVersion     string
	ProductType   string
	TransportType string
	TunnelState   string
	TunnelIP      string
}

func parseCoreDevicePhysicalInfos(data []byte) ([]coreDevicePhysicalInfo, error) {
	var output coreDeviceListOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("parse devicectl json: %w", err)
	}

	var devices []coreDevicePhysicalInfo
	for _, entry := range output.Result.Devices {
		if !isConnectedCoreDevicePhysical(entry) {
			continue
		}

		udid := strings.TrimSpace(entry.HardwareProperties.UDID)
		if udid == "" {
			continue
		}

		devices = append(devices, coreDevicePhysicalInfo{
			Identifier:  strings.TrimSpace(entry.Identifier),
			UDID:        udid,
			Name:        strings.TrimSpace(entry.DeviceProperties.Name),
			OSVersion:   strings.TrimSpace(entry.DeviceProperties.OSVersionNumber),
			ProductType: strings.TrimSpace(entry.HardwareProperties.ProductType),
			Transport:   strings.TrimSpace(entry.ConnectionProperties.TransportType),
			BootState:   strings.TrimSpace(entry.DeviceProperties.BootState),
			TunnelState: strings.TrimSpace(entry.ConnectionProperties.TunnelState),
			TunnelIP:    strings.TrimSpace(entry.ConnectionProperties.TunnelIPAddress),
		})
	}

	if devices == nil {
		devices = []coreDevicePhysicalInfo{}
	}

	return devices, nil
}

func runDevicectlJSON(args ...string) ([]byte, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("devicectl is only available on macOS")
	}

	fullArgs := append(args, "--json-output", "-")
	cmd := exec.Command("xcrun", append([]string{"devicectl"}, fullArgs...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("devicectl %s failed: %s", strings.Join(args, " "), message)
	}

	return output, nil
}

func getCoreDeviceDetails(udid string) (*coreDeviceDetails, error) {
	output, err := runDevicectlJSON("device", "info", "details", "--device", udid)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result coreDeviceEntry `json:"result"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse devicectl device details json: %w", err)
	}

	entry := result.Result
	entry.HardwareProperties.UDID = strings.TrimSpace(entry.HardwareProperties.UDID)
	if entry.HardwareProperties.UDID == "" {
		return nil, fmt.Errorf("device %s not found in devicectl details", udid)
	}

	return &coreDeviceDetails{
		Identifier:    strings.TrimSpace(entry.Identifier),
		UDID:          entry.HardwareProperties.UDID,
		Name:          strings.TrimSpace(entry.DeviceProperties.Name),
		OSVersion:     strings.TrimSpace(entry.DeviceProperties.OSVersionNumber),
		ProductType:   strings.TrimSpace(entry.HardwareProperties.ProductType),
		TransportType: strings.TrimSpace(entry.ConnectionProperties.TransportType),
		TunnelState:   strings.TrimSpace(entry.ConnectionProperties.TunnelState),
		TunnelIP:      strings.TrimSpace(entry.ConnectionProperties.TunnelIPAddress),
	}, nil
}

func isLocalNetworkCoreDevice(udid string) bool {
	details, err := getCoreDeviceDetails(udid)
	if err != nil {
		return false
	}

	return strings.EqualFold(details.TransportType, "localNetwork")
}

// isLocalNetworkCoreDevice reports whether this device is reachable over a
// CoreDevice localNetwork tunnel. For CoreDevice-discovered devices it consults
// the isWireless value cached at discovery, avoiding a `devicectl device info
// details` shell on every op (M1.7). Devices not discovered via CoreDevice (no
// CoreDeviceIdentifier) fall back to the devicectl query so their behavior is
// unchanged.
func (d *IOSDevice) isLocalNetworkCoreDevice() bool {
	if d.CoreDeviceIdentifier != "" {
		return d.isWireless
	}
	return isLocalNetworkCoreDevice(d.Udid)
}

func hasConnectedCoreDeviceTunnel(udid string) bool {
	details, err := getCoreDeviceDetails(udid)
	if err != nil {
		return false
	}

	return strings.EqualFold(details.TransportType, "localNetwork") &&
		strings.EqualFold(details.TunnelState, "connected") &&
		strings.TrimSpace(details.TunnelIP) != ""
}

func listCoreDeviceApps(udid string) ([]InstalledAppInfo, error) {
	output, err := runDevicectlJSON("device", "info", "apps", "--device", udid)
	if err != nil {
		return nil, err
	}

	var result coreDeviceAppsOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse devicectl apps json: %w", err)
	}

	apps := make([]InstalledAppInfo, 0, len(result.Result.Apps))
	for _, app := range result.Result.Apps {
		bundleID := strings.TrimSpace(app.BundleIdentifier)
		if bundleID == "" {
			continue
		}
		apps = append(apps, InstalledAppInfo{
			PackageName: bundleID,
			AppName:     strings.TrimSpace(app.Name),
			Version:     strings.TrimSpace(app.Version),
		})
	}

	return apps, nil
}

func launchCoreDeviceApp(udid, bundleID string) error {
	_, err := runDevicectlJSON("device", "process", "launch", "--device", udid, bundleID)
	return err
}

func openCoreDeviceURL(deviceID, url string) error {
	_, err := runDevicectlJSON("device", "process", "openURL", "--device", deviceID, url)
	if err != nil {
		return fmt.Errorf("failed to open url %q on CoreDevice %s: %w", url, deviceID, err)
	}
	return nil
}

func captureCoreDeviceScreenshot(udid string) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "mobilecli-coredevice-screenshot-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir for screenshot: %w", err)
	}
	defer os.RemoveAll(tempDir)

	path := filepath.Join(tempDir, "screenshot.png")
	cmd := exec.Command("xcrun", "devicectl", "device", "capture", "screenshot", "--device", udid, "--destination", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("devicectl capture screenshot failed: %s", message)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read devicectl screenshot: %w", err)
	}

	return data, nil
}

func getCoreDeviceDisplayInfo(udid string) (*ScreenSize, error) {
	output, err := runDevicectlJSON("device", "info", "displays", "--device", udid)
	if err != nil {
		return nil, err
	}

	var result coreDeviceDisplaysOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse devicectl displays json: %w", err)
	}

	for _, display := range result.Result.Displays {
		if !display.Primary || len(display.Bounds) != 2 || len(display.Bounds[1]) != 2 {
			continue
		}
		return &ScreenSize{
			Width:  display.Bounds[1][0],
			Height: display.Bounds[1][1],
			Scale:  display.PointScale,
		}, nil
	}

	return nil, fmt.Errorf("primary display not found")
}

func isConnectedCoreDevicePhysical(entry coreDeviceEntry) bool {
	if !strings.EqualFold(strings.TrimSpace(entry.HardwareProperties.Reality), "physical") {
		return false
	}

	if !strings.EqualFold(strings.TrimSpace(entry.ConnectionProperties.PairingState), "paired") {
		return false
	}

	booted := strings.EqualFold(strings.TrimSpace(entry.DeviceProperties.BootState), "booted")
	tunnelConnected := strings.EqualFold(strings.TrimSpace(entry.ConnectionProperties.TunnelState), "connected")
	transport := strings.TrimSpace(entry.ConnectionProperties.TransportType)

	return booted || tunnelConnected || transport == "usb" || transport == "localNetwork"
}

func listCoreDevicePhysicalDevices() ([]IOSDevice, error) {
	if runtime.GOOS != "darwin" {
		return []IOSDevice{}, nil
	}

	cmd := exec.Command("xcrun", "devicectl", "list", "devices", "--json-output", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("devicectl list devices failed: %s", message)
	}

	infos, err := parseCoreDevicePhysicalInfos(output)
	if err != nil {
		return nil, err
	}

	devices := make([]IOSDevice, 0, len(infos))
	for _, info := range infos {
		device, err := newIOSDevice(info.UDID, info.Name, info.OSVersion, info.ProductType)
		if err != nil {
			utils.Verbose("Warning: Failed to create iOS device from devicectl entry %s: %v", info.UDID, err)
			continue
		}
		// Retain both identifiers: ID()/Udid keeps returning the public hardware
		// UDID, while the CoreDevice identifier + tunnel IP are internal and used
		// for devicectl/tunnel operations.
		device.CoreDeviceIdentifier = info.Identifier
		device.TunnelIP = info.TunnelIP
		device.coreDeviceState = deriveCoreDeviceState(info.BootState, info.TunnelState)
		device.isWireless = strings.EqualFold(info.Transport, "localNetwork")
		devices = append(devices, device)
	}

	return devices, nil
}

// deriveCoreDeviceState maps CoreDevice boot/tunnel state onto the public device
// state vocabulary. A disconnected Apple TV must not be reported as online.
func deriveCoreDeviceState(bootState, tunnelState string) string {
	if strings.EqualFold(strings.TrimSpace(bootState), "booted") {
		return "online"
	}
	if strings.EqualFold(strings.TrimSpace(tunnelState), "connected") {
		return "online"
	}
	return "offline"
}

// installCoreDeviceApp installs an app bundle on a CoreDevice-discovered device
// using devicectl, avoiding any go-ios device lookup.
func installCoreDeviceApp(deviceID, path string) error {
	_, err := runDevicectlJSON("device", "install", "app", "--device", deviceID, path)
	return err
}

// uninstallCoreDeviceApp removes an app by bundle id on a CoreDevice-discovered
// device using devicectl.
func uninstallCoreDeviceApp(deviceID, bundleID string) error {
	_, err := runDevicectlJSON("device", "uninstall", "app", "--device", deviceID, bundleID)
	return err
}

type coreDeviceAppsRawOutput struct {
	Result struct {
		Apps []struct {
			BundleIdentifier string `json:"bundleIdentifier"`
			URL              string `json:"url"`
			Path             string `json:"path"`
		} `json:"apps"`
	} `json:"result"`
}

type coreDeviceProcessesOutput struct {
	Result struct {
		RunningProcesses []struct {
			ProcessIdentifier int    `json:"processIdentifier"`
			Executable        string `json:"executable"`
		} `json:"runningProcesses"`
	} `json:"result"`
}

// normalizeDevicectlPath strips a leading file:// scheme and any trailing slash
// so app bundle URLs and process executable paths can be prefix-compared.
func normalizeDevicectlPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "file://")
	return strings.TrimRight(p, "/")
}

// coreDeviceAppBundlePath resolves the on-device .app bundle path for a bundle id.
func coreDeviceAppBundlePath(deviceID, bundleID string) (string, error) {
	output, err := runDevicectlJSON("device", "info", "apps", "--device", deviceID)
	if err != nil {
		return "", err
	}
	return parseCoreDeviceAppBundlePath(output, deviceID, bundleID)
}

// parseCoreDeviceAppBundlePath extracts the on-device .app bundle path for a
// bundle id from `devicectl device info apps` JSON, preferring url over path and
// tolerating missing fields.
func parseCoreDeviceAppBundlePath(data []byte, deviceID, bundleID string) (string, error) {
	var result coreDeviceAppsRawOutput
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse devicectl apps json: %w", err)
	}

	for _, app := range result.Result.Apps {
		if strings.TrimSpace(app.BundleIdentifier) != bundleID {
			continue
		}
		if p := normalizeDevicectlPath(app.URL); p != "" {
			return p, nil
		}
		if p := normalizeDevicectlPath(app.Path); p != "" {
			return p, nil
		}
	}

	return "", fmt.Errorf("bundle %s not installed on device %s", bundleID, deviceID)
}

// matchesBundlePath reports whether an executable path belongs to the given app
// bundle. It matches on a "<bundlePath>/" boundary so a sibling bundle sharing a
// path prefix (e.g. .../Foo.app vs .../FooBar.app) is not treated as a match.
func matchesBundlePath(exe, bundlePath string) bool {
	if exe == "" || bundlePath == "" {
		return false
	}
	if exe == bundlePath {
		return true
	}
	return strings.HasPrefix(exe, strings.TrimRight(bundlePath, "/")+"/")
}

// resolveCoreDevicePID finds the running process id backing a bundle id by
// matching the app bundle path against the running-process executable paths.
func resolveCoreDevicePID(deviceID, bundleID string) (int, error) {
	bundlePath, err := coreDeviceAppBundlePath(deviceID, bundleID)
	if err != nil {
		return 0, err
	}

	output, err := runDevicectlJSON("device", "info", "processes", "--device", deviceID)
	if err != nil {
		return 0, err
	}

	return parseCoreDevicePID(output, deviceID, bundleID, bundlePath)
}

// parseCoreDevicePID selects the running process id whose executable path lives
// inside bundlePath from `devicectl device info processes` JSON.
func parseCoreDevicePID(data []byte, deviceID, bundleID, bundlePath string) (int, error) {
	var result coreDeviceProcessesOutput
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, fmt.Errorf("parse devicectl processes json: %w", err)
	}

	for _, proc := range result.Result.RunningProcesses {
		exe := normalizeDevicectlPath(proc.Executable)
		if matchesBundlePath(exe, bundlePath) {
			return proc.ProcessIdentifier, nil
		}
	}

	return 0, fmt.Errorf("no running process found for %s on device %s", bundleID, deviceID)
}

// terminateCoreDeviceApp terminates a running app by bundle id on a
// CoreDevice-discovered device: resolve the pid, then ask devicectl to
// terminate it.
func terminateCoreDeviceApp(deviceID, bundleID string) error {
	pid, err := resolveCoreDevicePID(deviceID, bundleID)
	if err != nil {
		return err
	}

	_, err = runDevicectlJSON("device", "process", "terminate", "--device", deviceID, "--pid", strconv.Itoa(pid))
	return err
}
