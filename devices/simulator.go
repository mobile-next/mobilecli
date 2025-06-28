package devices

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/utils"
	"github.com/otiai10/copy"
)

// AppInfo corresponds to the structure from plutil output
type AppInfo struct {
	CFBundleIdentifier  string `json:"CFBundleIdentifier"`
	CFBundleDisplayName string `json:"CFBundleDisplayName"`
	CFBundleVersion     string `json:"CFBundleVersion"`
}

// Simulator represents an iOS simulator device
type Simulator struct {
	Name    string `json:"name"`
	UDID    string `json:"udid"`
	State   string `json:"state"`
	Runtime string `json:"runtime"`
}

// SimulatorDevice wraps a Simulator to implement the AnyDevice interface
type SimulatorDevice struct {
	Simulator
}

func (s SimulatorDevice) ID() string         { return s.UDID }
func (s SimulatorDevice) Name() string       { return s.Simulator.Name }
func (s SimulatorDevice) Platform() string   { return "ios" }
func (s SimulatorDevice) DeviceType() string { return "simulator" }

func (s SimulatorDevice) TakeScreenshot() ([]byte, error) {
	return wda.TakeScreenshot()
}

// Reboot shuts down and then boots the iOS simulator.
func (s SimulatorDevice) Reboot() error {
	log.Printf("Attempting to reboot simulator: %s (%s)", s.Name(), s.UDID)

	// Shutdown the simulator
	log.Printf("SimulatorDevice: Shutting down %s...", s.UDID)
	output, err := runSimctl("shutdown", s.UDID)
	if err != nil {
		// Don't stop if shutdown fails for a simulator that might already be off
		log.Printf("SimulatorDevice: Shutdown command for %s may have failed (could be already off): %v\nOutput: %s", s.UDID, err, string(output))
	} else {
		log.Printf("SimulatorDevice: Shutdown successful for %s.", s.UDID)
	}

	// Boot the simulator
	log.Printf("SimulatorDevice: Booting %s...", s.UDID)
	output, err = runSimctl("boot", s.UDID)
	if err != nil {
		return fmt.Errorf("SimulatorDevice: failed to boot simulator %s: %v\nOutput: %s", s.UDID, err, string(output))
	}
	log.Printf("SimulatorDevice: Boot command successful for %s.", s.UDID)
	return nil
}

// runSimctl executes xcrun simctl with the provided arguments
func runSimctl(args ...string) ([]byte, error) {
	fullArgs := append([]string{"simctl"}, args...)
	cmd := exec.Command("xcrun", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute xcrun simctl command: %w", err)
	}
	return output, nil
}

// getSimulators executes 'xcrun simctl list --json' and returns the parsed response
func GetSimulators() ([]Simulator, error) {
	output, err := runSimctl("list", "--json")
	if err != nil {
		return nil, fmt.Errorf("failed to execute xcrun simctl list: %w", err)
	}

	var simulators map[string]interface{}
	if err := json.Unmarshal(output, &simulators); err != nil {
		return nil, fmt.Errorf("failed to parse simulator list JSON: %w", err)
	}

	devices, ok := simulators["devices"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected format in simulator list: devices not found or not a map")
	}

	var filteredDevices []Simulator

	for runtimeName, deviceList := range devices {
		deviceArray, ok := deviceList.([]interface{})
		if !ok {
			continue
		}

		for _, device := range deviceArray {
			deviceMap, ok := device.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := deviceMap["name"].(string)
			udid, _ := deviceMap["udid"].(string)
			state, _ := deviceMap["state"].(string)

			simulator := Simulator{
				Name:    name,
				UDID:    udid,
				State:   state,
				Runtime: runtimeName,
			}

			filteredDevices = append(filteredDevices, simulator)
		}
	}

	return filteredDevices, nil
}

func filterSimulatorsByState(simulators []Simulator, state string) []Simulator {
	var filteredDevices []Simulator
	for _, device := range simulators {
		if device.State == state {
			filteredDevices = append(filteredDevices, device)
		}
	}
	return filteredDevices
}

func GetBootedSimulators() ([]Simulator, error) {
	simulators, err := GetSimulators()
	if err != nil {
		return nil, err
	}

	return filterSimulatorsByState(simulators, "Booted"), nil
}

func (s SimulatorDevice) LaunchApp(bundleID string) error {
	_, err := runSimctl("launch", s.UDID, bundleID)
	return err
}

func (s SimulatorDevice) TerminateApp(bundleID string) error {
	_, err := runSimctl("terminate", s.UDID, bundleID)
	return err
}

func InstallApp(udid string, appPath string) error {
	log.Printf("Installing app from %s to simulator %s", appPath, udid)
	output, err := runSimctl("install", udid, appPath)
	if err != nil {
		return fmt.Errorf("failed to install app from %s: %v\n%s", appPath, err, output)
	}

	log.Printf("Successfully installed app from %s", appPath)
	return nil
}

func UninstallApp(udid string, bundleID string) error {
	log.Printf("Uninstalling app %s from simulator %s", bundleID, udid)
	output, err := runSimctl("uninstall", udid, bundleID)
	if err != nil {
		return fmt.Errorf("failed to uninstall app %s: %v\n%s", bundleID, err, output)
	}

	log.Printf("Successfully uninstalled app %s", bundleID)
	return nil
}

func (s SimulatorDevice) ListInstalledApps() (map[string]interface{}, error) {
	// use xcrun simctl
	output, err := runSimctl("listapps", s.UDID)
	if err != nil {
		return nil, fmt.Errorf("failed to list installed apps: %v\n%s", err, output)
	}

	// convert output to json
	cmd := exec.Command("plutil", "-convert", "json", "-o", "-", "-")
	cmd.Stdin = bytes.NewReader(output)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to convert output to JSON: %v\n%s", err, output)
	}

	// parse json
	var apps map[string]interface{}
	err = json.Unmarshal(output, &apps)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v\n%s", err, output)
	}

	return apps, nil
}

func (s SimulatorDevice) WaitUntilAppExists(bundleID string) error {
	installedApps, err := s.ListInstalledApps()
	if err != nil {
		return fmt.Errorf("failed to list installed apps: %v", err)
	}

	startTime := time.Now()
	for {
		_, ok := installedApps[bundleID]
		if ok {
			return nil
		}

		if time.Since(startTime) > 10*time.Second {
			return fmt.Errorf("app %s not found after 10 seconds", bundleID)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (s SimulatorDevice) DownloadWebDriverAgent() (string, error) {
	url := "https://github.com/appium/WebDriverAgent/releases/download/v9.13.0/WebDriverAgentRunner-Runner.zip"

	tmpFile, err := os.CreateTemp("", "wda-*.zip")
	log.Printf("Created temp file: %s", tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to download WebDriverAgent: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download WebDriverAgent: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download WebDriverAgent: status code %d", resp.StatusCode)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", err
	}

	defer tmpFile.Close()

	return tmpFile.Name(), nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %v", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %v", dst, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file %s: %v", src, err)
	}

	return nil
}

func (s SimulatorDevice) InstallWebDriverAgent() error {

	file, err := s.DownloadWebDriverAgent()
	if err != nil {
		return fmt.Errorf("failed to download WebDriverAgent: %v", err)
	}

	// defer os.Remove(file)

	log.Printf("Downloaded WebDriverAgent to %s", file)

	dir, err := utils.Unzip(file)
	if err != nil {
		return fmt.Errorf("failed to unzip WebDriverAgent: %v", err)
	}

	log.Printf("Unzipped WebDriverAgent to %s", dir)

	// copy frameworks into this directory
	frameworks := []string{
		"Testing.framework",
		"WebDriverAgentRunner.xctest",
		"XCTAutomationSupport.framework",
		"XCTest.framework",
		"XCTestCore.framework",
		"XCTestSupport.framework",
		"XCUIAutomation.framework",
		"XCUnit.framework",
	}

	files := []string{
		"libXCTestSwiftSupport.dylib",
	}

	xcodePath := "/Applications/Xcode.app/Contents/Developer"
	iPhoneSimulatorPath := xcodePath + "/Platforms/iPhoneSimulator.platform/Developer"
	frameworksPath := iPhoneSimulatorPath + "/Library/Frameworks/"

	// copy recursively all files in frameworks to the simulator
	for _, framework := range frameworks {
		src := frameworksPath + framework
		dst := dir + "/" + framework
		err = copy.Copy(src, dst)
		if err != nil {
			os.RemoveAll(dir)
			return fmt.Errorf("failed to copy framework %s: %v", src, err)
		}
	}

	for _, file := range files {
		src := iPhoneSimulatorPath + "/usr/lib/" + file
		dst := dir + "/" + file
		err = copyFile(src, dst)
		if err != nil {
			os.RemoveAll(dir)
			return fmt.Errorf("failed to copy file %s: %v", src, err)
		}
	}

	err = InstallApp(s.UDID, dir+"/WebDriverAgentRunner-Runner.app")
	if err != nil {
		return fmt.Errorf("failed to install WebDriverAgent: %v", err)
	}

	err = s.WaitUntilAppExists("com.facebook.WebDriverAgentRunner.xctrunner")
	if err != nil {
		return fmt.Errorf("failed to wait for WebDriverAgent to be installed: %v", err)
	}

	defer os.RemoveAll(dir)
	return nil
}

func (s SimulatorDevice) IsWebDriverAgentInstalled() (bool, error) {
	installedApps, err := s.ListInstalledApps()
	if err != nil {
		return false, err
	}

	webdriverPackageName := "com.facebook.WebDriverAgentRunner.xctrunner"
	_, ok := installedApps[webdriverPackageName]
	return ok, nil
}

func (s SimulatorDevice) StartAgent() error {
	_, err := wda.GetWebDriverAgentStatus()
	if err != nil {
		installed, err := s.IsWebDriverAgentInstalled()
		if err != nil {
			return err
		}

		if !installed {
			log.Printf("WebdriverAgent is not installed. Will try to install now")
			return fmt.Errorf("WebdriverAgent is not installed")
			// err = s.InstallWebDriverAgent()
			// if err != nil {
			// return fmt.Errorf("SimulatorDevice: failed to install WebDriverAgent: %v", err)
			// }
		}

		webdriverPackageName := "com.facebook.WebDriverAgentRunner.xctrunner"
		err = s.LaunchApp(webdriverPackageName)
		if err != nil {
			return err
		}

		err = wda.WaitForWebDriverAgent()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s SimulatorDevice) PressButton(key string) error {
	return wda.PressButton(key)
}

func (s SimulatorDevice) SendKeys(text string) error {
	return wda.SendKeys(text)
}

func (s SimulatorDevice) Tap(x, y int) error {
	return wda.Tap(x, y)
}

func (s *SimulatorDevice) OpenURL(url string) error {
	return exec.Command("xcrun", "simctl", "openurl", s.ID(), url).Run()
}

func (s *SimulatorDevice) ListApps() ([]InstalledAppInfo, error) {
	simctlCmd := exec.Command("xcrun", "simctl", "listapps", s.ID())
	plutilCmd := exec.Command("plutil", "-convert", "json", "-o", "-", "-r", "-")

	var err error
	plutilCmd.Stdin, err = simctlCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipe: %w", err)
	}

	var plutilOut bytes.Buffer
	plutilCmd.Stdout = &plutilOut

	if err := plutilCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start plutil: %w", err)
	}

	if err := simctlCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run simctl: %w", err)
	}

	if err := plutilCmd.Wait(); err != nil {
		return nil, fmt.Errorf("failed to wait for plutil: %w", err)
	}

	var output map[string]AppInfo
	if err := json.Unmarshal(plutilOut.Bytes(), &output); err != nil {
		return nil, fmt.Errorf("failed to parse plutil JSON output: %w", err)
	}

	var apps []InstalledAppInfo
	for _, app := range output {
		apps = append(apps, InstalledAppInfo{
			PackageName: app.CFBundleIdentifier,
			AppName:     app.CFBundleDisplayName,
			Version:     app.CFBundleVersion,
		})
	}

	return apps, nil
}

func (s Simulator) Info() (*FullDeviceInfo, error) {
	wdaSize, err := wda.GetWindowSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get window size from WDA: %w", err)
	}

	return &FullDeviceInfo{
		DeviceInfo: DeviceInfo{
			ID:       s.UDID,
			Name:     s.Name,
			Platform: "ios",
			Type:     "simulator",
		},
		ScreenSize: &ScreenSize{
			Width:  wdaSize.ScreenSize.Width,
			Height: wdaSize.ScreenSize.Height,
			Scale:  wdaSize.Scale,
		},
	}, nil
}
