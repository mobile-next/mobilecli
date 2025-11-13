package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"regexp"
	"time"

	"github.com/mobile-next/mobilecli/assets"
	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/devices/wda/mjpeg"
	"github.com/mobile-next/mobilecli/utils"
)

const (
	LOW_WDA_PORT    = 13001
	HIGH_WDA_PORT   = 13200
	LOW_MJPEG_PORT  = 13201
	HIGH_MJPEG_PORT = 13400
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
	wdaClient *wda.WdaClient
}

// parseSimulatorVersion parses iOS version from simulator runtime string
// e.g., "com.apple.CoreSimulator.SimRuntime.iOS-18-6" -> "18.6"
func parseSimulatorVersion(runtime string) string {
	// Use regex to extract iOS version from runtime string
	re := regexp.MustCompile(`iOS-(\d+)-(\d+)`)
	matches := re.FindStringSubmatch(runtime)
	if len(matches) == 3 {
		return matches[1] + "." + matches[2]
	}

	// Fallback: return the original runtime string if parsing fails
	return runtime
}

func (s SimulatorDevice) ID() string         { return s.UDID }
func (s SimulatorDevice) Name() string       { return s.Simulator.Name }
func (s SimulatorDevice) Platform() string   { return "ios" }
func (s SimulatorDevice) DeviceType() string { return "simulator" }
func (s SimulatorDevice) Version() string    { return parseSimulatorVersion(s.Runtime) }
func (s SimulatorDevice) State() string {
	if s.Simulator.State == "Booted" {
		return "online"
	}
	return "offline"
}

func (s SimulatorDevice) TakeScreenshot() ([]byte, error) {
	return s.wdaClient.TakeScreenshot()
}

// Reboot shuts down and then boots the iOS simulator.
func (s SimulatorDevice) Reboot() error {
	utils.Verbose("Attempting to reboot simulator: %s (%s)", s.Name(), s.UDID)

	// Shutdown the simulator
	utils.Verbose("SimulatorDevice: Shutting down %s...", s.UDID)
	output, err := runSimctl("shutdown", s.UDID)
	if err != nil {
		// Don't stop if shutdown fails for a simulator that might already be off
		utils.Verbose("SimulatorDevice: Shutdown command for %s may have failed (could be already off): %v\nOutput: %s", s.UDID, err, string(output))
	} else {
		utils.Verbose("SimulatorDevice: Shutdown successful for %s.", s.UDID)
	}

	// Boot the simulator
	utils.Verbose("SimulatorDevice: Booting %s...", s.UDID)
	output, err = runSimctl("boot", s.UDID)
	if err != nil {
		return fmt.Errorf("SimulatorDevice: failed to boot simulator %s: %v\nOutput: %s", s.UDID, err, string(output))
	}
	utils.Verbose("SimulatorDevice: Boot command successful for %s.", s.UDID)
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

// modifyPlistInput contains parameters for modifying a plist file
type modifyPlistInput struct {
	plistPath string
	key       string
	value     string
}

// modifyPlist modifies a plist file using plutil to add or replace a key-value pair
func modifyPlist(input modifyPlistInput) error {
	// try to replace first (if key exists)
	cmd := exec.Command("plutil", "-replace", input.key, "-string", input.value, input.plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// if replace failed, try to insert (key doesn't exist)
		cmd = exec.Command("plutil", "-insert", input.key, "-string", input.value, input.plistPath)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to modify plist: %w\n%s", err, output)
		}
	}
	return nil
}

// addIconToPlist adds the CFBundleIconFiles array to the plist
func addIconToPlist(plistPath string) error {
	// insert CFBundleIconFiles as array
	cmd := exec.Command("plutil", "-insert", "CFBundleIconFiles", "-array", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to insert CFBundleIconFiles: %w\n%s", err, output)
	}

	// insert AppIcon.png as first element in the array
	cmd = exec.Command("plutil", "-insert", "CFBundleIconFiles.0", "-string", "AppIcon.png", plistPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to insert AppIcon.png: %w\n%s", err, output)
	}

	return nil
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

// filterSimulatorsByDownloadsDirectory filters simulators that have been booted at least once
// by checking if the Downloads directory exists
func filterSimulatorsByDownloadsDirectory(simulators []Simulator) []Simulator {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var filteredDevices []Simulator
	for _, device := range simulators {
		downloadsPath := fmt.Sprintf("%s/Library/Developer/CoreSimulator/Devices/%s/data/Downloads", homeDir, device.UDID)
		if _, err := os.Stat(downloadsPath); err == nil {
			filteredDevices = append(filteredDevices, device)
		}
	}
	return filteredDevices
}

func (s SimulatorDevice) LaunchAppWithEnv(bundleID string, env map[string]string) error {
	// Build simctl command
	fullArgs := append([]string{"simctl", "launch"}, s.UDID, bundleID)
	cmd := exec.Command("xcrun", fullArgs...)

	// Set environment variables with SIMCTL_CHILD_ prefix for this command only
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SIMCTL_CHILD_%s=%s", key, value))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to launch app with env: %w", err)
	}

	_ = output // Suppress unused variable warning
	return nil
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
	utils.Verbose("Installing app from %s to simulator %s", appPath, udid)
	output, err := runSimctl("install", udid, appPath)
	if err != nil {
		return fmt.Errorf("failed to install app from %s: %v\n%s", appPath, err, output)
	}

	utils.Verbose("Successfully installed app from %s", appPath)
	return nil
}

func UninstallApp(udid string, bundleID string) error {
	utils.Verbose("Uninstalling app %s from simulator %s", bundleID, udid)
	output, err := runSimctl("uninstall", udid, bundleID)
	if err != nil {
		return fmt.Errorf("failed to uninstall app %s: %v\n%s", bundleID, err, output)
	}

	utils.Verbose("Successfully uninstalled app %s", bundleID)
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
	startTime := time.Now()
	for {
		installedApps, err := s.ListInstalledApps()
		if err != nil {
			return fmt.Errorf("failed to list installed apps: %v", err)
		}

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
	url := "https://github.com/appium/WebDriverAgent/releases/download/v9.15.1/WebDriverAgentRunner-Build-Sim-arm64.zip"

	tmpFile, err := os.CreateTemp("", "wda-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()

	utils.Verbose("Downloading WebDriverAgent to: %s", tmpFile.Name())

	err = utils.DownloadFile(url, tmpFile.Name())
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download WebDriverAgent: %w", err)
	}

	// log file size
	fileInfo, err := os.Stat(tmpFile.Name())
	if err == nil {
		utils.Verbose("Downloaded %d bytes", fileInfo.Size())
	}

	return tmpFile.Name(), nil
}

func (s SimulatorDevice) InstallWebDriverAgent() error {

	file, err := s.DownloadWebDriverAgent()
	if err != nil {
		return fmt.Errorf("failed to download WebDriverAgent: %v", err)
	}

	defer func() { _ = os.Remove(file) }()

	utils.Verbose("Downloaded WebDriverAgent to %s", file)

	dir, err := utils.Unzip(file)
	if err != nil {
		return fmt.Errorf("failed to unzip WebDriverAgent: %v", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()
	utils.Verbose("Unzipped WebDriverAgent to %s", dir)

	appDir := dir + "/WebDriverAgentRunner-Runner.app"
	infoPlistPath := appDir + "/Info.plist"

	// modify info.plist to add CFBundleDisplayName
	err = modifyPlist(modifyPlistInput{
		plistPath: infoPlistPath,
		key:       "CFBundleDisplayName",
		value:     "Mobile Next Kit",
	})
	if err != nil {
		return fmt.Errorf("failed to modify Info.plist: %w", err)
	}

	// write embedded icon file
	iconDest := appDir + "/AppIcon.png"
	err = os.WriteFile(iconDest, assets.AppIconData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write icon: %w", err)
	}

	// add icon configuration to plist
	err = addIconToPlist(infoPlistPath)
	if err != nil {
		return fmt.Errorf("failed to add icon to plist: %w", err)
	}

	err = InstallApp(s.UDID, appDir)
	if err != nil {
		return fmt.Errorf("failed to install WebDriverAgent: %w", err)
	}

	err = s.WaitUntilAppExists("com.facebook.WebDriverAgentRunner.xctrunner")
	if err != nil {
		return fmt.Errorf("failed to wait for WebDriverAgent to be installed: %w", err)
	}

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

func (s *SimulatorDevice) getState() (string, error) {
	simulators, err := GetSimulators()
	if err != nil {
		return "", err
	}

	for _, sim := range simulators {
		if sim.UDID == s.UDID {
			return sim.State, nil
		}
	}

	return "", fmt.Errorf("simulator %s not found", s.UDID)
}

func (s *SimulatorDevice) bootSimulator() error {
	utils.Verbose("Booting simulator %s...", s.UDID)
	output, err := runSimctl("boot", s.UDID)
	if err != nil {
		return fmt.Errorf("failed to boot simulator %s: %w\n%s", s.UDID, err, output)
	}
	utils.Verbose("Waiting for simulator to finish booting...")
	output, err = runSimctl("bootstatus", s.UDID)
	if err != nil {
		return fmt.Errorf("failed to wait for boot status %s: %w\n%s", s.UDID, err, output)
	}
	utils.Verbose("Simulator booted successfully")
	return nil
}

func (s *SimulatorDevice) StartAgent() error {
	// check simulator state and boot if needed
	state, err := s.getState()
	if err != nil {
		return fmt.Errorf("failed to get simulator state: %w", err)
	}

	switch state {
	case "Booted":
		// already booted, continue to WDA
	case "Shutdown":
		// boot the simulator
		if err := s.bootSimulator(); err != nil {
			return err
		}
	case "Booting":
		// simulator is already booting, just wait for it to finish
		utils.Verbose("Simulator is booting, waiting for boot to complete...")
		output, err := runSimctl("bootstatus", s.UDID)
		if err != nil {
			return fmt.Errorf("failed to wait for boot status: %w\n%s", err, output)
		}
		utils.Verbose("Simulator booted successfully")
	case "ShuttingDown":
		return fmt.Errorf("simulator is shutting down, please try again")
	default:
		return fmt.Errorf("unexpected simulator state: %s", state)
	}

	if currentPort, err := s.getWdaPort(); err == nil {
		// we ran this in the past already (between runs of mobilecli, it's still running on simulator)
		utils.Verbose("WebDriverAgent is already running on port %d", currentPort)

		// update our instance with new client
		s.wdaClient = wda.NewWdaClient(fmt.Sprintf("localhost:%d", currentPort))
		if _, err := s.wdaClient.GetStatus(); err == nil {
			// double check succeeded
			return nil // Already running and accessible
		}

		// TODO: it's running, but we failed to get status, we might as well kill the process and try again
		return err
	}

	installed, err := s.IsWebDriverAgentInstalled()
	if err != nil {
		return err
	}

	if !installed {
		utils.Verbose("WebdriverAgent is not installed. Will try to install now")
		err = s.InstallWebDriverAgent()
		if err != nil {
			return fmt.Errorf("SimulatorDevice: failed to install WebDriverAgent: %v", err)
		}

		// from here on, we assume wda is installed
	}

	// find available ports
	usePort, err := utils.FindAvailablePortInRange(LOW_WDA_PORT, HIGH_WDA_PORT)
	if err != nil {
		return fmt.Errorf("failed to find available USE_PORT: %w", err)
	}

	mjpegPort, err := utils.FindAvailablePortInRange(LOW_MJPEG_PORT, HIGH_MJPEG_PORT)
	if err != nil {
		return fmt.Errorf("failed to find available MJPEG_SERVER_PORT: %w", err)
	}

	utils.Verbose("Starting WebDriverAgent with USE_PORT=%d and MJPEG_SERVER_PORT=%d", usePort, mjpegPort)

	webdriverPackageName := "com.facebook.WebDriverAgentRunner.xctrunner"
	env := map[string]string{
		"MJPEG_SERVER_PORT": strconv.Itoa(mjpegPort),
		"USE_PORT":          strconv.Itoa(usePort),
	}

	err = s.LaunchAppWithEnv(webdriverPackageName, env)
	if err != nil {
		return err
	}

	// update WDA client to use the actual port
	s.wdaClient = wda.NewWdaClient(fmt.Sprintf("localhost:%d", usePort))

	err = s.wdaClient.WaitForAgent()
	if err != nil {
		return err
	}

	return nil
}

func (s SimulatorDevice) PressButton(key string) error {
	return s.wdaClient.PressButton(key)
}

func (s SimulatorDevice) SendKeys(text string) error {
	return s.wdaClient.SendKeys(text)
}

func (s SimulatorDevice) Tap(x, y int) error {
	return s.wdaClient.Tap(x, y)
}

func (s SimulatorDevice) LongPress(x, y int) error {
	return s.wdaClient.LongPress(x, y)
}

func (s SimulatorDevice) Swipe(x1, y1, x2, y2 int) error {
	return s.wdaClient.Swipe(x1, y1, x2, y2)
}

func (s SimulatorDevice) Gesture(actions []wda.TapAction) error {
	return s.wdaClient.Gesture(actions)
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

func (s *SimulatorDevice) Info() (*FullDeviceInfo, error) {
	wdaSize, err := s.wdaClient.GetWindowSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get window size from WDA: %w", err)
	}

	return &FullDeviceInfo{
		DeviceInfo: DeviceInfo{
			ID:       s.UDID,
			Name:     s.Simulator.Name,
			Platform: "ios",
			Type:     "simulator",
			Version:  parseSimulatorVersion(s.Runtime),
		},
		ScreenSize: &ScreenSize{
			Width:  wdaSize.ScreenSize.Width,
			Height: wdaSize.ScreenSize.Height,
			Scale:  wdaSize.Scale,
		},
	}, nil
}

func (s *SimulatorDevice) StartScreenCapture(format string, quality int, scale float64, callback func([]byte) bool) error {
	mjpegPort, err := s.getWdaMjpegPort()
	if err != nil {
		return fmt.Errorf("failed to get MJPEG port: %w", err)
	}

	// configure mjpeg framerate
	err = s.wdaClient.SetMjpegFramerate(DefaultMJPEGFramerate)
	if err != nil {
		return err
	}

	mjpegClient := mjpeg.NewWdaMjpegClient(fmt.Sprintf("http://localhost:%d", mjpegPort))
	return mjpegClient.StartScreenCapture(format, callback)
}

func findWdaProcessForDevice(deviceUDID string) (int, string, error) {
	cmd := exec.Command("/bin/ps", "-o", "pid,command", "-E", "-ww", "-e")
	output, err := cmd.Output()
	if err != nil {
		return 0, "", fmt.Errorf("failed to run ps command: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	devicePath := fmt.Sprintf("/Library/Developer/CoreSimulator/Devices/%s", deviceUDID)

	for _, line := range lines {
		if strings.Contains(line, devicePath) && strings.Contains(line, "WebDriverAgentRunner-Runner") {
			// Find the first space to separate PID from the rest
			spaceIndex := strings.Index(line, " ")
			if spaceIndex == -1 {
				continue
			}

			pidStr := strings.TrimSpace(line[:spaceIndex])
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}

			// The rest of the line contains command and environment
			processInfo := line[spaceIndex+1:]
			return pid, processInfo, nil
		}
	}

	return 0, "", fmt.Errorf("WebDriverAgent process not found for device %s", deviceUDID)
}

func extractEnvValue(output, envVar string) (string, error) {
	// Look for " ENVVAR=" pattern (space + envvar + equals)
	pattern := " " + envVar + "="
	pos := strings.Index(output, pattern)
	if pos == -1 {
		// Also check if it's at the beginning of the line
		pattern = envVar + "="
		if strings.HasPrefix(output, pattern) {
			pos = 0
		} else {
			return "", fmt.Errorf("%s not found in environment", envVar)
		}
	} else {
		pos++ // Skip the leading space
	}

	// Find the start of the value (after the =)
	valueStart := pos + len(envVar) + 1

	// Find the end of the value (next space)
	valueEnd := strings.Index(output[valueStart:], " ")
	if valueEnd == -1 {
		valueEnd = len(output)
	} else {
		valueEnd += valueStart
	}

	return output[valueStart:valueEnd], nil
}

func (s *SimulatorDevice) getWdaEnvPort(envVar string) (int, error) {
	_, processInfo, err := findWdaProcessForDevice(s.UDID)
	if err != nil {
		return 0, err
	}

	portStr, err := extractEnvValue(processInfo, envVar)
	if err != nil {
		return 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %s", envVar, portStr)
	}

	return port, nil
}

func (s SimulatorDevice) DumpSource() ([]ScreenElement, error) {
	return s.wdaClient.GetSourceElements()
}

func (s *SimulatorDevice) getWdaPort() (int, error) {
	return s.getWdaEnvPort("USE_PORT")
}

func (s *SimulatorDevice) getWdaMjpegPort() (int, error) {
	return s.getWdaEnvPort("MJPEG_SERVER_PORT")
}

func (s SimulatorDevice) InstallApp(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat path: %v", err)
	}

	if info.IsDir() {
		return InstallApp(s.UDID, path)
	}

	if strings.HasSuffix(path, ".zip") {
		tmpDir, err := utils.Unzip(path)
		if err != nil {
			return fmt.Errorf("failed to unzip: %v", err)
		}
		defer func() { _ = os.RemoveAll(tmpDir) }()

		entries, err := os.ReadDir(tmpDir)
		if err != nil {
			return fmt.Errorf("failed to read unzipped dir: %v", err)
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".app") {
				appPath := tmpDir + "/" + entry.Name()
				return InstallApp(s.UDID, appPath)
			}
		}

		return fmt.Errorf("no .app bundle found in zip file")
	}

	return InstallApp(s.UDID, path)
}

func (s SimulatorDevice) UninstallApp(packageName string) (*InstalledAppInfo, error) {
	installedApps, err := s.ListInstalledApps()
	if err != nil {
		return nil, fmt.Errorf("failed to list installed apps: %v", err)
	}

	if _, exists := installedApps[packageName]; !exists {
		return nil, fmt.Errorf("package %s is not installed", packageName)
	}

	appInfo := &InstalledAppInfo{
		PackageName: packageName,
	}

	err = UninstallApp(s.UDID, packageName)
	if err != nil {
		return nil, err
	}

	return appInfo, nil
}

// GetOrientation gets the current device orientation
func (s SimulatorDevice) GetOrientation() (string, error) {
	return s.wdaClient.GetOrientation()
}

// SetOrientation sets the device orientation
func (s SimulatorDevice) SetOrientation(orientation string) error {
	return s.wdaClient.SetOrientation(orientation)
}
