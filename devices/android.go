package devices

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

// AndroidDevice implements the ControllableDevice interface for Android devices
type AndroidDevice struct {
	id          string
	name        string
	version     string
	state       string // "online" or "offline"
	transportID string // adb transport ID (e.g., "emulator-5554"), only set for online devices
}

func (d *AndroidDevice) ID() string {
	return d.id
}

func (d *AndroidDevice) Name() string {
	return d.name
}

func (d *AndroidDevice) Version() string {
	return d.version
}

func (d *AndroidDevice) Platform() string {
	return "android"
}

func (d *AndroidDevice) DeviceType() string {
	// check transportID for online devices, or state for offline
	if strings.HasPrefix(d.transportID, "emulator-") || d.state == "offline" {
		return "emulator"
	} else {
		return "real"
	}
}

func (d *AndroidDevice) State() string {
	return d.state
}

func getAndroidSdkPath() string {
	sdkPath := os.Getenv("ANDROID_HOME")
	if sdkPath != "" {
		if _, err := os.Stat(sdkPath); err == nil {
			return sdkPath
		}
	}

	// try default Android SDK location on macOS
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		defaultPath := filepath.Join(homeDir, "Library", "Android", "sdk")
		if _, err := os.Stat(defaultPath); err == nil {
			return defaultPath
		}
	}

	// try default Android SDK location on Windows
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			defaultPath := filepath.Join(localAppData, "Android", "Sdk")
			if _, err := os.Stat(defaultPath); err == nil {
				return defaultPath
			}
		}

		// fallback to USERPROFILE on Windows
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			defaultPath := filepath.Join(userProfile, "AppData", "Local", "Android", "Sdk")
			if _, err := os.Stat(defaultPath); err == nil {
				return defaultPath
			}
		}
	}

	return ""
}

func getAdbPath() string {
	sdkPath := getAndroidSdkPath()
	if sdkPath != "" {
		adbPath := filepath.Join(sdkPath, "platform-tools", "adb")
		if runtime.GOOS == "windows" {
			adbPath += ".exe"
		}

		return adbPath
	}

	// best effort, look in path
	return "adb"
}

func getEmulatorPath() string {
	sdkPath := getAndroidSdkPath()
	if sdkPath != "" {
		emulatorPath := filepath.Join(sdkPath, "emulator", "emulator")
		if runtime.GOOS == "windows" {
			emulatorPath += ".exe"
		}
		if _, err := os.Stat(emulatorPath); err == nil {
			return emulatorPath
		}
	}

	// best effort, look in path
	return "emulator"
}

// getAdbIdentifier returns the correct device identifier for adb commands
// uses transportID for online devices (e.g., "emulator-5554"), or id for offline
func (d *AndroidDevice) getAdbIdentifier() string {
	if d.transportID != "" {
		return d.transportID
	}
	return d.id
}

func (d *AndroidDevice) runAdbCommand(args ...string) ([]byte, error) {
	deviceID := d.getAdbIdentifier()
	cmdArgs := append([]string{"-s", deviceID}, args...)
	cmd := exec.Command(getAdbPath(), cmdArgs...)
	return cmd.CombinedOutput()
}

// getDisplayCount counts the number of displays on the device
func (d *AndroidDevice) getDisplayCount() int {
	output, err := d.runAdbCommand("shell", "dumpsys", "SurfaceFlinger", "--display-id")
	if err != nil {
		return 1 // assume single display on error
	}

	count := 0
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Display ") {
			count++
		}
	}

	return count
}

// parseDisplayIdFromCmdDisplay extracts display ID from "cmd display get-displays" output (Android 11+)
func parseDisplayIdFromCmdDisplay(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		// look for lines like "Display id X, ... state ON, ... uniqueId "..."
		if strings.HasPrefix(line, "Display id ") &&
			strings.Contains(line, ", state ON,") &&
			strings.Contains(line, ", uniqueId ") {
			re := regexp.MustCompile(`uniqueId "([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) == 2 {
				return strings.TrimPrefix(matches[1], "local:")
			}
		}
	}
	return ""
}

// parseDisplayIdFromDumpsysViewport extracts display ID from dumpsys DisplayViewport entries
func parseDisplayIdFromDumpsysViewport(dumpsys string) string {
	re := regexp.MustCompile(`DisplayViewport\{type=INTERNAL[^}]*isActive=true[^}]*uniqueId='([^']+)'`)
	matches := re.FindStringSubmatch(dumpsys)
	if len(matches) == 2 {
		return strings.TrimPrefix(matches[1], "local:")
	}
	return ""
}

// parseDisplayIdFromDumpsysState extracts display ID from dumpsys display state entries
func parseDisplayIdFromDumpsysState(dumpsys string) string {
	re := regexp.MustCompile(`Display Id=(\d+)[\s\S]*?Display State=ON`)
	matches := re.FindStringSubmatch(dumpsys)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

// getFirstDisplayId finds the first active display's unique ID
func (d *AndroidDevice) getFirstDisplayId() string {
	// try using cmd display get-displays (Android 11+)
	output, err := d.runAdbCommand("shell", "cmd", "display", "get-displays")
	if err == nil {
		if id := parseDisplayIdFromCmdDisplay(string(output)); id != "" {
			return id
		}
	}

	// fallback: parse dumpsys display for display info (compatible with older Android versions)
	output, err = d.runAdbCommand("shell", "dumpsys", "display")
	if err != nil {
		return ""
	}

	dumpsys := string(output)

	// try DisplayViewport entries with isActive=true and type=INTERNAL
	if id := parseDisplayIdFromDumpsysViewport(dumpsys); id != "" {
		return id
	}

	// final fallback: look for active display with state ON
	return parseDisplayIdFromDumpsysState(dumpsys)
}

// captureScreenshot captures screenshot with optional display ID
func (d *AndroidDevice) captureScreenshot(displayID string) ([]byte, error) {
	args := []string{"exec-out", "screencap", "-p"}
	if displayID != "" {
		args = append(args, "-d", displayID)
	}
	byteData, err := d.runAdbCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}
	return byteData, nil
}

func (d *AndroidDevice) TakeScreenshot() ([]byte, error) {
	displayCount := d.getDisplayCount()

	if displayCount <= 1 {
		// backward compatibility for android 10 and below, and for single display devices
		return d.captureScreenshot("")
	}

	// find the first display that is turned on, and capture that one
	displayID := d.getFirstDisplayId()
	if displayID == "" {
		// no idea why, but we have displayCount >= 2, yet we failed to parse
		// let's go with screencap's defaults and hope for the best
		return d.captureScreenshot("")
	}

	return d.captureScreenshot(displayID)
}

func (d *AndroidDevice) LaunchApp(bundleID string) error {
	output, err := d.runAdbCommand("shell", "monkey", "-p", bundleID, "-c", "android.intent.category.LAUNCHER", "1")
	if err != nil {
		return fmt.Errorf("failed to launch app %s: %v\nOutput: %s", bundleID, err, string(output))
	}

	return nil
}

func (d *AndroidDevice) TerminateApp(bundleID string) error {
	output, err := d.runAdbCommand("shell", "am", "force-stop", bundleID)
	if err != nil {
		return fmt.Errorf("failed to terminate app %s: %v\nOutput: %s", bundleID, err, string(output))
	}

	return nil
}

// Reboot reboots the Android device/emulator using `adb reboot`.
func (d *AndroidDevice) Reboot() error {
	_, err := d.runAdbCommand("reboot")
	if err != nil {
		return err
	}

	return nil
}

// Shutdown shuts down the Android emulator
func (d *AndroidDevice) Shutdown() error {
	if d.DeviceType() != "emulator" {
		return fmt.Errorf("shutdown is only supported for emulators")
	}

	if d.state == "offline" {
		return fmt.Errorf("emulator is already offline")
	}

	// use emu kill command for graceful shutdown
	_, err := d.runAdbCommand("emu", "kill")
	if err != nil {
		return fmt.Errorf("failed to shutdown emulator: %w", err)
	}

	d.state = "offline"
	d.transportID = ""
	return nil
}

// Tap simulates a tap at (x, y) on the Android device.
func (d *AndroidDevice) Tap(x, y int) error {
	_, err := d.runAdbCommand("shell", "input", "tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	if err != nil {
		return err
	}

	return nil
}

// LongPress simulates a long press at (x, y) on the Android device.
func (d *AndroidDevice) LongPress(x, y int) error {
	_, err := d.runAdbCommand("shell", "input", "swipe", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y), fmt.Sprintf("%d", x), fmt.Sprintf("%d", y), "500")
	if err != nil {
		return err
	}

	return nil
}

// Swipe simulates a swipe gesture from (x1, y1) to (x2, y2) on the Android device with 1000ms duration.
func (d *AndroidDevice) Swipe(x1, y1, x2, y2 int) error {
	_, err := d.runAdbCommand("shell", "input", "swipe", fmt.Sprintf("%d", x1), fmt.Sprintf("%d", y1), fmt.Sprintf("%d", x2), fmt.Sprintf("%d", y2), "1000")
	if err != nil {
		return err
	}

	return nil
}

// Gesture performs a sequence of touch actions on the Android device
func (d *AndroidDevice) Gesture(actions []wda.TapAction) error {

	x := 0
	y := 0

	for _, action := range actions {
		var cmd []string

		if action.Type == "pause" {
			time.Sleep(time.Duration(action.Duration) * time.Millisecond)
			continue
		}

		switch action.Type {
		case "pointerDown":
			cmd = []string{"shell", "input", "touchscreen", "motionevent", "down", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)}
		case "pointerMove":
			x = action.X
			y = action.Y
			cmd = []string{"shell", "input", "touchscreen", "motionevent", "move", fmt.Sprintf("%d", action.X), fmt.Sprintf("%d", action.Y)}
		case "pointerUp":
			cmd = []string{"shell", "input", "touchscreen", "motionevent", "up", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y)}
		default:
			return fmt.Errorf("unsupported gesture action type: %s", action.Type)
		}

		_, err := d.runAdbCommand(cmd...)
		if err != nil {
			return fmt.Errorf("failed to execute gesture action %s: %v", action.Type, err)
		}
	}

	return nil
}

func parseAdbDevicesOutput(output string) []ControllableDevice {
	var devices []ControllableDevice

	lines := strings.Split(output, "\n")
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		parts := strings.Fields(line)
		if len(parts) == 2 {
			transportID := parts[0]
			status := parts[1]
			if status == "device" {
				deviceID := transportID

				// for emulators, use AVD name as the consistent ID
				if strings.HasPrefix(transportID, "emulator-") {
					avdName := getAVDName(transportID)
					if avdName != "" {
						deviceID = avdName
					}
				}

				devices = append(devices, &AndroidDevice{
					id:          deviceID,
					transportID: transportID,
					name:        getAndroidDeviceName(transportID),
					version:     getAndroidDeviceVersion(transportID),
					state:       "online",
				})
			}
		}
	}

	return devices
}

// getAVDName returns the AVD name for an emulator, or empty string if not an emulator
func getAVDName(transportID string) string {
	avdCmd := exec.Command(getAdbPath(), "-s", transportID, "shell", "getprop", "ro.boot.qemu.avd_name")
	avdOutput, err := avdCmd.CombinedOutput()
	if err == nil && len(avdOutput) > 0 {
		avdName := strings.TrimSpace(string(avdOutput))
		return avdName
	}
	return ""
}

func getAndroidDeviceName(deviceID string) string {
	// try getting AVD name first (for emulators)
	avdName := getAVDName(deviceID)
	if avdName != "" {
		return strings.ReplaceAll(avdName, "_", " ")
	}

	// fall back to product model
	modelCmd := exec.Command(getAdbPath(), "-s", deviceID, "shell", "getprop", "ro.product.model")
	modelOutput, err := modelCmd.CombinedOutput()
	if err == nil && len(modelOutput) > 0 {
		return strings.TrimSpace(string(modelOutput))
	}

	return deviceID
}

func getAndroidDeviceVersion(deviceID string) string {
	versionCmd := exec.Command(getAdbPath(), "-s", deviceID, "shell", "getprop", "ro.build.version.release")
	versionOutput, err := versionCmd.CombinedOutput()
	if err == nil && len(versionOutput) > 0 {
		return strings.TrimSpace(string(versionOutput))
	}

	return ""
}

// GetAndroidDevices retrieves a list of connected Android devices
func GetAndroidDevices() ([]ControllableDevice, error) {
	command := exec.Command(getAdbPath(), "devices")
	output, err := command.CombinedOutput()
	if err != nil {
		status := command.ProcessState.ExitCode()
		if status < 0 {
			utils.Verbose("Failed running 'adb devices', is ANDROID_HOME set correctly?")
			return []ControllableDevice{}, nil
		}

		return nil, fmt.Errorf("failed to run 'adb devices': %v", err)
	}

	androidDevices := parseAdbDevicesOutput(string(output))
	return androidDevices, nil
}

func (d *AndroidDevice) StartAgent(config StartAgentConfig) error {
	// if device is offline, return error - user should use 'device boot' command
	if d.state == "offline" {
		return fmt.Errorf("device is offline, use 'mobilecli device boot --device %s' to start the emulator", d.id)
	}

	// android doesn't need an agent to be started for online devices
	return nil
}

// matchesAVDName checks if a device name matches an AVD name (pure function)
func matchesAVDName(avdName, deviceName string) bool {
	normalizedAVD := strings.ReplaceAll(avdName, "_", " ")
	return normalizedAVD == deviceName || avdName == deviceName
}

// waitForEmulatorBootComplete waits for an emulator to appear and be fully booted
func (d *AndroidDevice) waitForEmulatorBootComplete(ctx context.Context, avdName string) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("emulator boot cancelled: %w", ctx.Err())
		case <-ticker.C:
			// check if emulator is in device list
			devices, err := GetAndroidDevices()
			if err != nil {
				continue // keep trying
			}

			for _, device := range devices {
				// check if this is our emulator by matching the AVD name
				if device.Platform() == "android" && device.DeviceType() == "emulator" {
					// device.ID() now returns the AVD name for emulators
					if device.ID() == avdName || matchesAVDName(avdName, device.Name()) {
						// found our emulator, check if it's fully booted
						// need to get the transport ID for the boot check
						if androidDev, ok := device.(*AndroidDevice); ok {
							transportID := androidDev.transportID
							if transportID == "" {
								transportID = androidDev.id
							}
							bootComplete, _ := d.checkBootComplete(transportID)
							if bootComplete {
								return androidDev.transportID, nil
							}
						}
					}
				}
			}
		}
	}
}

// Boot launches an offline Android emulator and waits for it to be ready
func (d *AndroidDevice) Boot() error {
	if d.state != "offline" {
		return fmt.Errorf("emulator is already running")
	}
	utils.Verbose("Starting Android emulator: %s", d.id)

	// create context with timeout for the boot wait process
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// launch emulator in background without context (so it persists after function returns)
	cmd := exec.Command(getEmulatorPath(), "-netdelay", "none", "-netspeed", "full", "-avd", d.id, "-qt-hide-window")
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start emulator: %w", err)
	}

	// monitor context cancellation to clean up the process only on timeout
	go func() {
		<-ctx.Done()
		if cmd.Process != nil && ctx.Err() == context.DeadlineExceeded {
			utils.Verbose("Boot timeout exceeded, killing emulator process")
			_ = cmd.Process.Kill()
		}
	}()

	utils.Verbose("Waiting for emulator to boot...")

	// wait for emulator to boot and get its actual device ID
	deviceID, err := d.waitForEmulatorBootComplete(ctx, d.id)
	if err != nil {
		// if boot failed, kill the emulator process
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return err
	}

	utils.Verbose("Emulator booted successfully with transport ID: %s", deviceID)
	// update our transport ID to the actual emulator-XXXX ID
	// the device ID (d.id) is already set to the AVD name and should not change
	d.transportID = deviceID
	d.state = "online"
	return nil
}

// checkBootComplete checks if an emulator has finished booting
func (d *AndroidDevice) checkBootComplete(deviceID string) (bool, error) {
	cmd := exec.Command(getAdbPath(), "-s", deviceID, "shell", "getprop", "sys.boot_completed")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(output)) == "1", nil
}

func (d *AndroidDevice) PressButton(key string) error {
	keyMap := map[string]string{
		"HOME":        "KEYCODE_HOME",
		"BACK":        "KEYCODE_BACK",
		"VOLUME_UP":   "KEYCODE_VOLUME_UP",
		"VOLUME_DOWN": "KEYCODE_VOLUME_DOWN",
		"ENTER":       "KEYCODE_ENTER",
		"DPAD_CENTER": "KEYCODE_DPAD_CENTER",
		"DPAD_UP":     "KEYCODE_DPAD_UP",
		"DPAD_DOWN":   "KEYCODE_DPAD_DOWN",
		"DPAD_LEFT":   "KEYCODE_DPAD_LEFT",
		"DPAD_RIGHT":  "KEYCODE_DPAD_RIGHT",
		"BACKSPACE":   "KEYCODE_DEL",
		"APP_SWITCH":  "KEYCODE_APP_SWITCH",
		"POWER":       "KEYCODE_POWER",
	}

	keycode, exists := keyMap[key]
	if !exists {
		return fmt.Errorf("AndroidDevice: unsupported button key: %s", key)
	}

	output, err := d.runAdbCommand("shell", "input", "keyevent", keycode)
	if err != nil {
		return fmt.Errorf("AndroidDevice: failed to press %s button: %v\nOutput: %s", key, err, string(output))
	}

	return nil
}

// isDeviceKitInstalled checks if DeviceKit is installed on the device
func (d *AndroidDevice) isDeviceKitInstalled() bool {
	appPath, err := d.GetAppPath("com.mobilenext.devicekit")
	return err == nil && appPath != ""
}

// isAscii checks if text contains only ASCII characters
func isAscii(text string) bool {
	for _, char := range text {
		if char > 127 {
			return false
		}
	}
	return true
}

// escapeShellText escapes shell special characters
func escapeShellText(text string) string {
	// escape all shell special characters that could be used for injection
	specialChars := `\'"`+ "`" + `
|&;()<>{}[]$*?`
	result := ""
	for _, char := range text {
		if strings.ContainsRune(specialChars, char) {
			result += "\\"
		}
		result += string(char)
	}
	return result
}

func (d *AndroidDevice) SendKeys(text string) error {
	if text == "" {
		// bailing early, so we don't run adb shell with empty string.
		// this happens when you prompt with a simple "submit".
		return nil
	}

	switch text {
	case "\b":
		return d.PressButton("BACKSPACE")
	case "\n":
		return d.PressButton("ENTER")
	}

	if isAscii(text) {
		// adb shell input only supports ascii characters. and
		// some of the keys have to be escaped.
		escapedText := escapeShellText(text)
		_, err := d.runAdbCommand("shell", "input", "text", escapedText)
		return err
	}

	// try sending over clipboard if DeviceKit is installed
	if d.isDeviceKitInstalled() {
		// ensure clipboard is always cleared, even on failure
		defer func() {
			_, _ = d.runAdbCommand("shell", "am", "broadcast", "-a", "devicekit.clipboard.clear", "-n", "com.mobilenext.devicekit/.ClipboardBroadcastReceiver")
		}()

		// encode text as base64
		base64Text := base64.StdEncoding.EncodeToString([]byte(text))

		// send clipboard over and immediately paste it
		_, err := d.runAdbCommand("shell", "am", "broadcast", "-a", "devicekit.clipboard.set", "-e", "encoding", "base64", "-e", "text", base64Text, "-n", "com.mobilenext.devicekit/.ClipboardBroadcastReceiver")
		if err != nil {
			return fmt.Errorf("failed to set clipboard: %w", err)
		}

		_, err = d.runAdbCommand("shell", "input", "keyevent", "KEYCODE_PASTE")
		if err != nil {
			return fmt.Errorf("failed to paste: %w", err)
		}

		return nil
	}

	return fmt.Errorf("non-ASCII text is not supported on Android, please install mobilenext devicekit, see https://github.com/mobile-next/devicekit-android")
}

func (d *AndroidDevice) OpenURL(url string) error {
	output, err := d.runAdbCommand("shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", url)
	if err != nil {
		return fmt.Errorf("failed to open URL %s: %v\nOutput: %s", url, err, string(output))
	}

	return nil
}

func (d *AndroidDevice) ListApps() ([]InstalledAppInfo, error) {
	output, err := d.runAdbCommand("shell", "cmd", "package", "query-activities", "-a", "android.intent.action.MAIN", "-c", "android.intent.category.LAUNCHER")
	if err != nil {
		return nil, fmt.Errorf("failed to query launcher activities: %v", err)
	}

	lines := strings.Split(string(output), "\n")

	var packageNames []string
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "packageName=") {
			packageName := strings.TrimPrefix(line, "packageName=")
			if !seen[packageName] {
				seen[packageName] = true
				packageNames = append(packageNames, packageName)
			}
		}
	}

	var apps []InstalledAppInfo
	for _, packageName := range packageNames {
		apps = append(apps, InstalledAppInfo{
			PackageName: packageName,
		})
	}

	return apps, nil
}

func (d *AndroidDevice) Info() (*FullDeviceInfo, error) {

	// run adb shell wm size
	output, err := d.runAdbCommand("shell", "wm", "size")
	if err != nil {
		return nil, fmt.Errorf("failed to get screen size: %v", err)
	}

	// split result by space, and then take 2nd argument split by "x"
	screenSize := strings.Split(string(output), " ")
	pair := strings.Trim(screenSize[len(screenSize)-1], "\r\n")
	parts := strings.SplitN(pair, "x", 2)

	widthInt, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get screen size: %v", err)
	}

	heightInt, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to get screen size: %v", err)
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
			Width:  widthInt,
			Height: heightInt,
			Scale:  1,
		},
	}, nil
}

func (d *AndroidDevice) GetAppPath(packageName string) (string, error) {
	output, err := d.runAdbCommand("shell", "pm", "path", packageName)
	if err != nil {
		// best effort (pm path will return error code 1)
		return "", nil
	}

	// remove the "package:" prefix
	appPath := strings.TrimPrefix(string(output), "package:")
	// trim all whitespace including \r\n (CRLF on Windows)
	appPath = strings.TrimSpace(appPath)
	return appPath, nil
}

func (d *AndroidDevice) StartScreenCapture(config ScreenCaptureConfig) error {
	if config.Format != "mjpeg" && config.Format != "avc" {
		return fmt.Errorf("unsupported format: %s, only 'mjpeg' and 'avc' are supported", config.Format)
	}

	if config.OnProgress != nil {
		config.OnProgress("Installing Agent")
	}

	utils.Verbose("Ensuring DeviceKit is installed...")
	err := d.EnsureDeviceKitInstalled()
	if err != nil {
		return fmt.Errorf("failed to ensure DeviceKit is installed: %v", err)
	}

	appPath, err := d.GetAppPath("com.mobilenext.devicekit")
	if err != nil {
		return fmt.Errorf("failed to get app path: %v", err)
	}

	var serverClass string
	if config.Format == "mjpeg" {
		serverClass = "com.mobilenext.devicekit.MjpegServer"
	} else {
		serverClass = "com.mobilenext.devicekit.AvcServer"
	}

	if config.OnProgress != nil {
		config.OnProgress("Starting Agent")
	}

	utils.Verbose("Starting %s with app path: %s", serverClass, appPath)
	cmdArgs := append([]string{"-s", d.getAdbIdentifier()}, "exec-out", fmt.Sprintf("CLASSPATH=%s", appPath), "app_process", "/system/bin", serverClass, "--quality", fmt.Sprintf("%d", config.Quality), "--scale", fmt.Sprintf("%.2f", config.Scale), "--fps", fmt.Sprintf("%d", config.FPS))
	utils.Verbose("Running command: %s %s", getAdbPath(), strings.Join(cmdArgs, " "))
	cmd := exec.Command(getAdbPath(), cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %v", serverClass, err)
	}

	// Read bytes from the command output and send to callback
	buffer := make([]byte, 65536)
	for {
		n, err := stdout.Read(buffer)
		if err != nil {
			break
		}

		if n > 0 {
			// Send bytes to callback, break if it returns false
			if !config.OnData(buffer[:n]) {
				break
			}
		}
	}

	_ = cmd.Process.Kill()
	return nil
}

func (d *AndroidDevice) installPackage(apkPath string) error {
	output, err := d.runAdbCommand("install", apkPath)
	if err != nil {
		return fmt.Errorf("failed to install package: %v\nOutput: %s", err, string(output))
	}

	if strings.Contains(string(output), "Success") {
		return nil
	}

	return fmt.Errorf("installation failed: %s", string(output))
}

func (d *AndroidDevice) EnsureDeviceKitInstalled() error {
	packageName := "com.mobilenext.devicekit"

	appPath, err := d.GetAppPath(packageName)
	if err != nil {
		return fmt.Errorf("failed to check if %s is installed: %v", packageName, err)
	}

	if appPath != "" {
		// already installed, we have a path to .apk
		return nil
	}

	utils.Verbose("DeviceKit not installed, downloading and installing...")

	downloadURL, err := utils.GetLatestReleaseDownloadURL("mobile-next/devicekit-android")
	if err != nil {
		return fmt.Errorf("failed to get download URL: %v", err)
	}
	utils.Verbose("Downloading APK from: %s", downloadURL)

	tempDir, err := os.MkdirTemp("", "devicekit-android-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	apkPath := filepath.Join(tempDir, "devicekit.apk")

	if err := utils.DownloadFile(downloadURL, apkPath); err != nil {
		return fmt.Errorf("failed to download APK: %v", err)
	}

	utils.Verbose("Installing APK...")
	if err := d.installPackage(apkPath); err != nil {
		return fmt.Errorf("failed to install APK: %v", err)
	}

	appPath, err = d.GetAppPath(packageName)
	if err != nil {
		return fmt.Errorf("failed to verify installation: %v", err)
	}

	if appPath == "" {
		return fmt.Errorf("package %s was not installed successfully", packageName)
	}

	utils.Verbose("DeviceKit successfully installed")
	return nil
}

type uiAutomatorXmlNode struct {
	XMLName     xml.Name             `xml:"node"`
	Class       string               `xml:"class,attr"`
	Text        string               `xml:"text,attr"`
	Bounds      string               `xml:"bounds,attr"`
	Hint        string               `xml:"hint,attr"`
	Focused     string               `xml:"focused,attr"`
	ContentDesc string               `xml:"content-desc,attr"`
	ResourceID  string               `xml:"resource-id,attr"`
	Nodes       []uiAutomatorXmlNode `xml:"node"`
}

type uiAutomatorXml struct {
	XMLName  xml.Name           `xml:"hierarchy"`
	RootNode uiAutomatorXmlNode `xml:"node"`
}

func (d *AndroidDevice) getScreenElementRect(bounds string) types.ScreenElementRect {
	re := regexp.MustCompile(`^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$`)
	matches := re.FindStringSubmatch(bounds)

	if len(matches) != 5 {
		return types.ScreenElementRect{}
	}

	left, _ := strconv.Atoi(matches[1])
	top, _ := strconv.Atoi(matches[2])
	right, _ := strconv.Atoi(matches[3])
	bottom, _ := strconv.Atoi(matches[4])

	return types.ScreenElementRect{
		X:      left,
		Y:      top,
		Width:  right - left,
		Height: bottom - top,
	}
}

func (d *AndroidDevice) collectElements(node uiAutomatorXmlNode) []types.ScreenElement {
	var elements []types.ScreenElement

	// recursively process child nodes
	for _, childNode := range node.Nodes {
		childElements := d.collectElements(childNode)
		elements = append(elements, childElements...)
	}

	// process current node if it has text, content-desc, or hint
	if node.Text != "" || node.ContentDesc != "" || node.Hint != "" {
		rect := d.getScreenElementRect(node.Bounds)

		// only include elements with positive width and height
		if rect.Width > 0 && rect.Height > 0 {
			element := types.ScreenElement{
				Type: node.Class,
				Text: &node.Text,
				Rect: rect,
			}

			// set label from content-desc or hint
			if node.ContentDesc != "" {
				element.Label = &node.ContentDesc
			} else if node.Hint != "" {
				element.Label = &node.Hint
			}

			// set focused if true
			if node.Focused == "true" {
				focused := true
				element.Focused = &focused
			}

			// set identifier from resource-id
			if node.ResourceID != "" {
				element.Identifier = &node.ResourceID
			}

			// default type if class is empty
			if element.Type == "" {
				element.Type = "text"
			}

			elements = append(elements, element)
		}
	}

	return elements
}

func (d *AndroidDevice) getUiAutomatorDump() (string, error) {
	for tries := 0; tries < 10; tries++ {
		output, err := d.runAdbCommand("exec-out", "uiautomator", "dump", "/dev/tty")
		if err != nil {
			return "", fmt.Errorf("failed to run uiautomator dump: %w", err)
		}

		dump := string(output)

		// check for known error condition
		if strings.Contains(dump, "null root node returned by UiTestAutomationBridge") {
			continue
		}

		// find the start of XML content
		xmlStart := strings.Index(dump, "<?xml")
		if xmlStart == -1 {
			return "", fmt.Errorf("no XML content found in uiautomator dump")
		}

		return dump[xmlStart:], nil
	}

	return "", fmt.Errorf("failed to get UIAutomator XML after 10 tries")
}

func (d *AndroidDevice) DumpSourceRaw() (interface{}, error) {
	// get the XML dump from uiautomator
	xmlContent, err := d.getUiAutomatorDump()
	if err != nil {
		return nil, fmt.Errorf("failed to get uiautomator dump: %w", err)
	}

	return xmlContent, nil
}

func (d *AndroidDevice) DumpSource() ([]ScreenElement, error) {
	// get the raw XML dump
	rawData, err := d.DumpSourceRaw()
	if err != nil {
		return nil, err
	}

	xmlContent, ok := rawData.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected type for raw XML data")
	}

	// parse the XML
	var uiXml uiAutomatorXml
	if err := xml.Unmarshal([]byte(xmlContent), &uiXml); err != nil {
		return nil, fmt.Errorf("failed to parse uiautomator XML: %w", err)
	}

	// collect elements from the hierarchy
	elements := d.collectElements(uiXml.RootNode)

	return elements, nil
}

func (d *AndroidDevice) InstallApp(path string) error {
	output, err := d.runAdbCommand("install", "-r", path)
	if err != nil {
		return fmt.Errorf("failed to install app: %v\nOutput: %s", err, string(output))
	}

	if strings.Contains(string(output), "Success") {
		return nil
	}

	return fmt.Errorf("installation failed: %s", string(output))
}

func (d *AndroidDevice) UninstallApp(packageName string) (*InstalledAppInfo, error) {
	appInfo := &InstalledAppInfo{
		PackageName: packageName,
	}

	output, err := d.runAdbCommand("uninstall", packageName)
	if err != nil {
		return nil, fmt.Errorf("failed to uninstall app: %v\nOutput: %s", err, string(output))
	}

	if !strings.Contains(string(output), "Success") {
		return nil, fmt.Errorf("uninstallation failed: %s", string(output))
	}

	return appInfo, nil
}

// GetOrientation gets the current device orientation
func (d *AndroidDevice) GetOrientation() (string, error) {
	output, err := d.runAdbCommand("shell", "settings", "get", "system", "user_rotation")
	if err != nil {
		return "", fmt.Errorf("failed to get orientation: %v", err)
	}

	rotationStr := strings.TrimSpace(string(output))
	rotation, err := strconv.Atoi(rotationStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse orientation value '%s': %v", rotationStr, err)
	}

	// convert Android rotation values to string
	switch rotation {
	case 0, 2:
		return "portrait", nil
	case 1, 3:
		return "landscape", nil
	default:
		return "portrait", nil // default to portrait
	}
}

// SetOrientation sets the device orientation
func (d *AndroidDevice) SetOrientation(orientation string) error {
	if orientation != "portrait" && orientation != "landscape" {
		return fmt.Errorf("invalid orientation value '%s', must be 'portrait' or 'landscape'", orientation)
	}

	var androidRotation int
	switch orientation {
	case "portrait":
		androidRotation = 0
	case "landscape":
		androidRotation = 1 // landscape left
	}

	// disable auto-rotation first
	_, err := d.runAdbCommand("shell", "settings", "put", "system", "accelerometer_rotation", "0")
	if err != nil {
		return fmt.Errorf("failed to disable auto-rotation: %v", err)
	}

	// set the orientation
	_, err = d.runAdbCommand("shell", "content", "insert", "--uri", "content://settings/system", "--bind", "name:s:user_rotation", "--bind", fmt.Sprintf("value:i:%d", androidRotation))
	if err != nil {
		return fmt.Errorf("failed to set orientation: %v", err)
	}

	return nil
}
