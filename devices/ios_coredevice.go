package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		devices = append(devices, device)
	}

	return devices, nil
}
