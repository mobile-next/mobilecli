package devices

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices/devicekit"
	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
)




// AppInfo corresponds to the structure from plutil output
type AppInfo struct {
	CFBundleIdentifier  string `json:"CFBundleIdentifier"`
	CFBundleDisplayName string `json:"CFBundleDisplayName"`
	CFBundleVersion     string `json:"CFBundleVersion"`
}

// devicePlist represents the structure of device.plist
type devicePlist struct {
	UDID       string `plist:"UDID"`
	Name       string `plist:"name"`
	Runtime    string `plist:"runtime"`
	State      int    `plist:"state"`
	DeviceType string `plist:"deviceType"`
}

// Simulator represents an iOS simulator device
type Simulator struct {
	Name       string `json:"name"`
	UDID       string `json:"udid"`
	State      string `json:"state"`
	Runtime    string `json:"runtime"`
	DeviceType string `json:"deviceType"`
}

// SimulatorDevice wraps a Simulator to implement the AnyDevice interface
type SimulatorDevice struct {
	Simulator
	controlClient IOSControl
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
	return s.controlClient.TakeScreenshot()
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
			Name:       device.Name,
			UDID:       device.UDID,
			State:      stateStr,
			Runtime:    device.Runtime,
			DeviceType: device.DeviceType,
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

func getDeviceKitSimFilename(arch string) string {
	if arch == "amd64" {
		return "devicekit-ios-Sim-x86_64.zip"
	}
	return "devicekit-ios-Sim-arm64.zip"
}

func getDeviceKitSimDownloadURL(arch string) string {
	return "https://github.com/mobile-next/devicekit-ios/releases/download/0.0.3/" + getDeviceKitSimFilename(arch)
}

func (s SimulatorDevice) downloadDeviceKitFromGitHub() (string, error) {
	url := getDeviceKitSimDownloadURL(runtime.GOARCH)

	tmpFile, err := os.CreateTemp("", "devicekit-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	_ = tmpFile.Close()

	utils.Verbose("Downloading DeviceKit to: %s", tmpFile.Name())

	if err := utils.DownloadFile(url, tmpFile.Name()); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download DeviceKit: %w", err)
	}

	if fi, err := os.Stat(tmpFile.Name()); err == nil {
		utils.Verbose("Downloaded %d bytes", fi.Size())
	}

	return tmpFile.Name(), nil
}

func (s SimulatorDevice) InstallDeviceKit(onProgress func(string)) error {
	var file string
	var shouldCleanup bool

	// try local path first (override via MOBILECLI_DEVICEKIT_PATH env var)
	if localBase := os.Getenv("MOBILECLI_DEVICEKIT_PATH"); localBase != "" {
		localPath := filepath.Join(localBase, getDeviceKitSimFilename(runtime.GOARCH))
		if _, err := os.Stat(localPath); err == nil {
			utils.Verbose("Using local DeviceKit from: %s", localPath)
			file = localPath
		} else {
			utils.Verbose("Local DeviceKit not found at: %s", localPath)
		}
	}

	// fall back to GitHub download
	if file == "" {
		if onProgress != nil {
			onProgress("Downloading DeviceKit")
		}

		downloaded, err := s.downloadDeviceKitFromGitHub()
		if err != nil {
			return fmt.Errorf("failed to download DeviceKit: %w", err)
		}
		file = downloaded
		shouldCleanup = true
	}

	if shouldCleanup {
		defer func() { _ = os.Remove(file) }()
	}

	if onProgress != nil {
		onProgress("Installing DeviceKit")
	}

	dir, err := utils.Unzip(file)
	if err != nil {
		return fmt.Errorf("failed to unzip DeviceKit: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	utils.Verbose("Unzipped DeviceKit to %s", dir)

	// find the .app bundle inside the zip
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read unzipped dir: %w", err)
	}

	var appDir string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".app") {
			appDir = filepath.Join(dir, entry.Name())
			break
		}
	}

	if appDir == "" {
		return fmt.Errorf("no .app bundle found in DeviceKit zip")
	}

	if err := InstallApp(s.UDID, appDir); err != nil {
		return fmt.Errorf("failed to install DeviceKit: %w", err)
	}

	if err := s.waitForDeviceKitInstalled(); err != nil {
		return fmt.Errorf("DeviceKit did not appear after install: %w", err)
	}

	return nil
}

func (s SimulatorDevice) IsDeviceKitInstalled() (bool, error) {
	installedApps, err := s.ListInstalledApps()
	if err != nil {
		return false, err
	}

	for id := range installedApps {
		if strings.Contains(id, "devicekit-ios") && strings.Contains(id, "UITests.xctrunner") {
			return true, nil
		}
	}
	return false, nil
}

func (s SimulatorDevice) waitForDeviceKitInstalled() error {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := s.IsDeviceKitInstalled()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("DeviceKit not found after install (timeout)")
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
		// already booted, continue
	case "Shutdown":
		return fmt.Errorf("simulator is offline, use 'mobilecli device boot --device %s' to start the simulator", s.UDID)
	case "Booting":
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

	if config.OnProgress != nil {
		config.OnProgress("Starting DeviceKit")
	}

	_, err = s.ensureSimulatorDeviceKit(config.OnProgress)
	return err
}

func (s SimulatorDevice) PressButton(key string) error {
	return s.controlClient.PressButton(key)
}

func (s SimulatorDevice) SendKeys(text string) error {
	return s.controlClient.SendKeys(text)
}

func (s SimulatorDevice) Tap(x, y int) error {
	return s.controlClient.Tap(x, y)
}

func (s SimulatorDevice) LongPress(x, y, duration int) error {
	return s.controlClient.LongPress(x, y, duration)
}

func (s SimulatorDevice) Swipe(x1, y1, x2, y2 int) error {
	return s.controlClient.Swipe(x1, y1, x2, y2)
}

func (s SimulatorDevice) Gesture(actions []types.TapAction) error {
	return s.controlClient.Gesture(actions)
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

func (s *SimulatorDevice) GetForegroundApp() (*ForegroundAppInfo, error) {
	activeApp, err := s.controlClient.GetForegroundApp()
	if err != nil {
		return nil, fmt.Errorf("failed to get active app info: %w", err)
	}

	// get all installed apps to enrich with version information
	apps, err := s.ListApps()
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

	// if app not found in list (e.g., system app), return basic info from DeviceKit
	return &ForegroundAppInfo{
		PackageName: activeApp.BundleID,
		AppName:     activeApp.Name,
		Version:     "",
	}, nil
}

func (s *SimulatorDevice) Info() (*FullDeviceInfo, error) {
	size, err := s.controlClient.GetWindowSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get window size: %w", err)
	}

	return &FullDeviceInfo{
		DeviceInfo: DeviceInfo{
			ID:       s.UDID,
			Name:     s.Simulator.Name,
			Platform: "ios",
			Type:     "simulator",
			Version:  parseSimulatorVersion(s.Runtime),
			State:    s.State(),
			Model:    s.Simulator.DeviceType,
		},
		ScreenSize: &ScreenSize{
			Width:  size.ScreenSize.Width,
			Height: size.ScreenSize.Height,
			Scale:  size.Scale,
		},
	}, nil
}

func (s *SimulatorDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	// avc+replay-kit requires BroadcastExtension which is unavailable on simulators
	if config.Format == "avc+replay-kit" {
		log.Warn("avc+replay-kit is not available on simulators: requires a real device with BroadcastExtension running")
		return fmt.Errorf("avc+replay-kit is not supported on simulators")
	}

	if config.OnProgress != nil {
		config.OnProgress("Checking DeviceKit status")
	}

	if _, err := s.ensureSimulatorDeviceKit(config.OnProgress); err != nil {
		return fmt.Errorf("failed to ensure DeviceKit on simulator: %w", err)
	}

	fps := config.FPS
	if fps == 0 {
		fps = DefaultFramerate
	}

	if config.Format == "avc" {
		if config.OnProgress != nil {
			config.OnProgress("Starting H.264 stream")
		}
		return s.controlClient.StartH264Stream(fps, config.Quality, config.Scale, config.OnData)
	}

	// default: mjpeg
	if config.OnProgress != nil {
		config.OnProgress("Starting MJPEG stream")
	}
	return s.controlClient.StartMjpegStream(fps, config.OnData)
}

// ensureSimulatorDeviceKit checks if DeviceKit is running on the simulator and starts it if not.
// DeviceKit on a simulator binds directly to the host's network, so no port forwarding is needed.
// On success s.controlClient is set to the active DeviceKit client.
func (s *SimulatorDevice) ensureSimulatorDeviceKit(onProgress func(string)) (int, error) {
	const port = deviceKitHTTPPort // 12004

	client := devicekit.NewClient("localhost", port)
	if err := client.HealthCheck(); err == nil {
		utils.Verbose("DeviceKit already running on simulator at port %d", port)
		s.controlClient = client
		return port, nil
	}

	installedApps, err := s.ListInstalledApps()
	if err != nil {
		return 0, fmt.Errorf("failed to list installed apps: %w", err)
	}

	var bundleID string
	for id := range installedApps {
		if strings.Contains(id, "devicekit-ios") && strings.Contains(id, "UITests.xctrunner") {
			bundleID = id
			break
		}
	}

	if bundleID == "" {
		// DeviceKit not installed — auto-install from GitHub
		if err := s.InstallDeviceKit(onProgress); err != nil {
			return 0, fmt.Errorf("DeviceKit not installed and auto-install failed: %w", err)
		}

		// re-read apps after install
		installedApps, err = s.ListInstalledApps()
		if err != nil {
			return 0, fmt.Errorf("failed to list installed apps after install: %w", err)
		}
		for id := range installedApps {
			if strings.Contains(id, "devicekit-ios") && strings.Contains(id, "UITests.xctrunner") {
				bundleID = id
				break
			}
		}
		if bundleID == "" {
			return 0, fmt.Errorf("DeviceKit not found after installation")
		}
	}

	if onProgress != nil {
		onProgress("Launching DeviceKit on simulator")
	}

	if err := s.LaunchApp(bundleID); err != nil {
		return 0, fmt.Errorf("failed to launch DeviceKit on simulator: %w", err)
	}

	if onProgress != nil {
		onProgress("Waiting for DeviceKit to start")
	}

	if err := client.WaitForReady(20 * time.Second); err != nil {
		return 0, fmt.Errorf("DeviceKit did not become ready on simulator: %w", err)
	}

	s.controlClient = client
	return port, nil
}


func (s SimulatorDevice) DumpSource() ([]ScreenElement, error) {
	return s.controlClient.GetSourceElements()
}

func (s SimulatorDevice) DumpSourceRaw() (interface{}, error) {
	return s.controlClient.GetSourceRaw()
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
	return s.controlClient.GetOrientation()
}

// SetOrientation sets the device orientation
func (s SimulatorDevice) SetOrientation(orientation string) error {
	return s.controlClient.SetOrientation(orientation)
}
