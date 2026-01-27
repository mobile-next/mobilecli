package devices

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/assets"
	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/devices/wda/mjpeg"
	"github.com/mobile-next/mobilecli/utils"
	"howett.net/plist"
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

// devicePlist represents the structure of device.plist
type devicePlist struct {
	UDID    string `plist:"UDID"`
	Name    string `plist:"name"`
	Runtime string `plist:"runtime"`
	State   int    `plist:"state"`
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

// getSimulators reads simulator information from the filesystem
func GetSimulators() ([]Simulator, error) {
	if runtime.GOOS != "darwin" {
		return []Simulator{}, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	devicesPath := filepath.Join(homeDir, "Library", "Developer", "CoreSimulator", "Devices")
	entries, err := os.ReadDir(devicesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read devices directory: %w", err)
	}

	var simulators []Simulator

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		plistPath := filepath.Join(devicesPath, entry.Name(), "device.plist")
		data, err := os.ReadFile(plistPath)
		if err != nil {
			// skip devices without device.plist
			continue
		}

		var device devicePlist
		if _, err := plist.Unmarshal(data, &device); err != nil {
			// skip devices with invalid plist
			continue
		}

		// convert state integer to string
		// state 1 = Shutdown (offline)
		// state 3 = Booted (online)
		stateStr := "Shutdown"
		if device.State == 3 {
			stateStr = "Booted"
		}

		simulator := Simulator{
			Name:    device.Name,
			UDID:    device.UDID,
			State:   stateStr,
			Runtime: device.Runtime,
		}

		simulators = append(simulators, simulator)
	}

	return simulators, nil
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
	output, err := runSimctl("listapps", s.UDID)
	if err != nil {
		return nil, fmt.Errorf("failed to list installed apps: %v\n%s", err, output)
	}

	var apps map[string]interface{}
	err = utils.ConvertPlistToJSON(output, &apps)
	if err != nil {
		return nil, err
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

func getWebdriverAgentFilename(arch string) string {
	if arch == "amd64" {
		return "WebDriverAgentRunner-Build-Sim-x86_64.zip"
	}
	return "WebDriverAgentRunner-Build-Sim-arm64.zip"
}

func getWebdriverAgentDownloadUrl(arch string) string {
	filename := getWebdriverAgentFilename(arch)
	return "https://github.com/appium/WebDriverAgent/releases/download/v10.2.5/" + filename
}

func (s SimulatorDevice) downloadWebDriverAgentFromGitHub() (string, error) {
	url := getWebdriverAgentDownloadUrl(runtime.GOARCH)

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

	fileInfo, err := os.Stat(tmpFile.Name())
	if err == nil {
		utils.Verbose("Downloaded %d bytes", fileInfo.Size())
	}

	return tmpFile.Name(), nil
}

func (s SimulatorDevice) InstallWebDriverAgent(onProgress func(string)) error {
	var file string
	var shouldCleanup bool

	// try local file first
	wdaPath := os.Getenv("MOBILECLI_WDA_PATH")
	if wdaPath != "" {
		filename := getWebdriverAgentFilename(runtime.GOARCH)
		localPath := filepath.Join(wdaPath, filename)

		if _, err := os.Stat(localPath); err == nil {
			utils.Verbose("Using local WebDriverAgent from: %s", localPath)
			file = localPath
			shouldCleanup = false
		} else {
			utils.Verbose("Local WebDriverAgent not found at: %s", localPath)
		}
	}

	// fall back to GitHub download
	if file == "" {
		if onProgress != nil {
			onProgress("Downloading WebDriverAgent")
		}

		downloadedFile, err := s.downloadWebDriverAgentFromGitHub()
		if err != nil {
			return fmt.Errorf("failed to download WebDriverAgent: %w", err)
		}
		file = downloadedFile
		shouldCleanup = true
	}

	if shouldCleanup {
		defer func() { _ = os.Remove(file) }()
	}

	if onProgress != nil {
		onProgress("Installing WebDriverAgent")
	}

	dir, err := utils.Unzip(file)
	if err != nil {
		return fmt.Errorf("failed to unzip WebDriverAgent: %w", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()
	utils.Verbose("Unzipped WebDriverAgent to %s", dir)

	appDir := dir + "/WebDriverAgentRunner-Runner.app"
	infoPlistPath := appDir + "/Info.plist"

	// modify info.plist to add CFBundleDisplayName
	err = utils.ModifyPlist(utils.ModifyPlistInput{
		PlistPath: infoPlistPath,
		Key:       "CFBundleDisplayName",
		Value:     "Mobile Next Kit",
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
	err = utils.AddBundleIconFilesToPlist(infoPlistPath)
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

// Boot boots the iOS simulator
func (s *SimulatorDevice) Boot() error {
	state, err := s.getState()
	if err != nil {
		return fmt.Errorf("failed to get simulator state: %w", err)
	}

	if state == "Booted" {
		return fmt.Errorf("simulator is already running")
	}

	if state == "Booting" {
		utils.Verbose("Simulator is already booting, waiting for boot to complete...")
		output, err := runSimctl("bootstatus", s.UDID)
		if err != nil {
			return fmt.Errorf("failed to wait for boot status: %w\n%s", err, output)
		}

		utils.Verbose("Simulator booted successfully")
		s.Simulator.State = "Booted"
		return nil
	}

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
	s.Simulator.State = "Booted"
	return nil
}

// Shutdown shuts down the iOS simulator
func (s *SimulatorDevice) Shutdown() error {
	state, err := s.getState()
	if err != nil {
		return fmt.Errorf("failed to get simulator state: %w", err)
	}

	if state == "Shutdown" {
		return fmt.Errorf("simulator is already offline")
	}

	utils.Verbose("Shutting down simulator %s...", s.UDID)
	output, err := runSimctl("shutdown", s.UDID)
	if err != nil {
		return fmt.Errorf("failed to shutdown simulator %s: %w\n%s", s.UDID, err, output)
	}

	utils.Verbose("Simulator shut down successfully")
	s.Simulator.State = "Shutdown"
	return nil
}

func (s *SimulatorDevice) StartAgent(config StartAgentConfig) error {
	// check simulator state - it must be booted
	state, err := s.getState()
	if err != nil {
		return fmt.Errorf("failed to get simulator state: %w", err)
	}

	switch state {
	case "Booted":
		// already booted, continue to WDA
	case "Shutdown":
		// simulator is offline, user should boot it first
		return fmt.Errorf("simulator is offline, use 'mobilecli device boot --device %s' to start the simulator", s.UDID)
	case "Booting":
		// simulator is already booting, just wait for it to finish
		if config.OnProgress != nil {
			config.OnProgress("Waiting for Simulator to boot")
		}

		utils.Verbose("Simulator is booting, waiting for boot to complete...")
		output, err := runSimctl("bootstatus", s.UDID)
		if err != nil {
			return fmt.Errorf("failed to wait for boot status: %w\n%s", err, output)
		}

		utils.Verbose("Simulator booted successfully")
		s.Simulator.State = "Booted"
	case "ShuttingDown":
		return fmt.Errorf("simulator is shutting down, please try again")
	default:
		return fmt.Errorf("unexpected simulator state: %s", state)
	}

	if currentPort, err := s.getWdaPort(); err == nil {
		// we ran this in the past already (between runs of mobilecli, it's still running on simulator)

		// check if we already have a client pointing to the same port
		expectedURL := fmt.Sprintf("localhost:%d", currentPort)
		if s.wdaClient != nil {
			// check if the existing client is already pointing to the same port
			if _, err := s.wdaClient.GetStatus(); err == nil {
				return nil // already connected to the right port
			}
		}

		utils.Verbose("WebDriverAgent is already running on port %d", currentPort)

		// create new client or update with new port
		s.wdaClient = wda.NewWdaClient(expectedURL)
		if _, err := s.wdaClient.GetStatus(); err == nil {
			// double check succeeded
			return nil // Already running and accessible
		}

		// TODO: it's running, but we failed to get status, we might as well kill the process and try again
		return fmt.Errorf("WebDriverAgent is running but not accessible on port %d", currentPort)
	} else {
		utils.Verbose("Failed to get existing WDA port: %v", err)
	}

	installed, err := s.IsWebDriverAgentInstalled()
	if err != nil {
		return err
	}

	if !installed {
		utils.Verbose("WebdriverAgent is not installed. Will try to install now")
		if config.OnProgress != nil {
			config.OnProgress("Installing Agent on Simulator")
		}

		err = s.InstallWebDriverAgent(config.OnProgress)
		if err != nil {
			return fmt.Errorf("SimulatorDevice: failed to install WebDriverAgent: %v", err)
		}

		// from here on, we assume wda is installed
	}

	if config.OnProgress != nil {
		config.OnProgress("Starting Agent")
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
		"USE_PORT":          strconv.Itoa(usePort),
		"MJPEG_SERVER_PORT": strconv.Itoa(mjpegPort),
	}

	err = s.LaunchAppWithEnv(webdriverPackageName, env)
	if err != nil {
		return err
	}

	// update WDA client to use the actual port
	s.wdaClient = wda.NewWdaClient(fmt.Sprintf("localhost:%d", usePort))

	if config.OnProgress != nil {
		config.OnProgress("Waiting for agent to start")
	}

	err = s.wdaClient.WaitForAgent()
	if err != nil {
		// terminate the WDA process if it failed to start
		_ = s.TerminateApp(webdriverPackageName)
		return err
	}

	// wait 1 second after pressing home, so we make sure wda is in the background
	_ = s.wdaClient.PressButton("HOME")
	time.Sleep(1 * time.Second)

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

func (s SimulatorDevice) LongPress(x, y, duration int) error {
	return s.wdaClient.LongPress(x, y, duration)
}

func (s SimulatorDevice) Swipe(x1, y1, x2, y2 int) error {
	return s.wdaClient.Swipe(x1, y1, x2, y2)
}

func (s SimulatorDevice) Gesture(actions []wda.TapAction) error {
	return s.wdaClient.Gesture(actions)
}

func (s *SimulatorDevice) OpenURL(url string) error {
	// #nosec G204 -- udid is controlled, no shell interpretation
	return exec.Command("xcrun", "simctl", "openurl", s.ID(), url).Run()
}

func (s *SimulatorDevice) ListApps() ([]InstalledAppInfo, error) {
	output, err := runSimctl("listapps", s.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w\n%s", err, output)
	}

	var appsMap map[string]AppInfo
	err = utils.ConvertPlistToJSON(output, &appsMap)
	if err != nil {
		return nil, err
	}

	var apps []InstalledAppInfo
	for _, app := range appsMap {
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
			State:    s.State(),
		},
		ScreenSize: &ScreenSize{
			Width:  wdaSize.ScreenSize.Width,
			Height: wdaSize.ScreenSize.Height,
			Scale:  wdaSize.Scale,
		},
	}, nil
}

func (s *SimulatorDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	mjpegPort, err := s.getWdaMjpegPort()
	if err != nil {
		return fmt.Errorf("failed to get MJPEG port: %w", err)
	}

	// configure mjpeg framerate
	fps := config.FPS
	if fps == 0 {
		fps = DefaultFramerate
	}
	err = s.wdaClient.SetMjpegFramerate(fps)
	if err != nil {
		return err
	}

	if config.OnProgress != nil {
		config.OnProgress("Starting video stream")
	}

	mjpegClient := mjpeg.NewWdaMjpegClient(fmt.Sprintf("http://localhost:%d", mjpegPort))
	return mjpegClient.StartScreenCapture(config.Format, config.OnData)
}

type ProcessInfo struct {
	PID     int
	Command string
}

// listAllProcesses returns a list of all running processes with their PIDs and command info
func listAllProcesses() ([]ProcessInfo, error) {
	cmd := exec.Command("/bin/ps", "-o", "pid,command", "-E", "-ww", "-e")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run ps command: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	processes := make([]ProcessInfo, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		// find the first space to separate PID from the rest
		spaceIndex := strings.Index(line, " ")
		if spaceIndex == -1 {
			continue
		}

		pidStr := strings.TrimSpace(line[:spaceIndex])
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// the rest of the line contains command and environment
		command := line[spaceIndex+1:]
		processes = append(processes, ProcessInfo{
			PID:     pid,
			Command: command,
		})
	}

	return processes, nil
}

func findWdaProcessForDevice(deviceUDID string) (int, string, error) {
	processes, err := listAllProcesses()
	if err != nil {
		return 0, "", err
	}

	devicePath := fmt.Sprintf("/Library/Developer/CoreSimulator/Devices/%s", deviceUDID)

	for _, proc := range processes {
		if strings.Contains(proc.Command, devicePath) && strings.Contains(proc.Command, "WebDriverAgentRunner-Runner") {
			return proc.PID, proc.Command, nil
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
	pid, processInfo, err := findWdaProcessForDevice(s.UDID)
	if err != nil {
		utils.Verbose("Could not find WDA process: %v", err)
		return 0, err
	}

	utils.Verbose("Found WDA process PID=%d", pid)

	portStr, err := extractEnvValue(processInfo, envVar)
	if err != nil {
		utils.Verbose("Could not extract %s from process info: %v", envVar, err)
		return 0, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %s", envVar, portStr)
	}

	utils.Verbose("Extracted %s=%d from WDA process", envVar, port)
	return port, nil
}

func (s SimulatorDevice) DumpSource() ([]ScreenElement, error) {
	return s.wdaClient.GetSourceElements()
}

func (s SimulatorDevice) DumpSourceRaw() (interface{}, error) {
	return s.wdaClient.GetSourceRaw()
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
