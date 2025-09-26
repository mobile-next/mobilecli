package devices

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/mobile-next/mobilecli/types"
	"github.com/mobile-next/mobilecli/utils"
)

// AndroidDevice implements the ControllableDevice interface for Android devices
type AndroidDevice struct {
	id      string
	name    string
	version string
}

func (d AndroidDevice) ID() string {
	return d.id
}

func (d AndroidDevice) Name() string {
	return d.name
}

func (d AndroidDevice) Version() string {
	return d.version
}

func (d AndroidDevice) Platform() string {
	return "android"
}

func (d AndroidDevice) DeviceType() string {
	if strings.HasPrefix(d.id, "emulator-") {
		return "emulator"
	} else {
		return "real"
	}
}

func getAdbPath() string {
	adbPath := os.Getenv("ANDROID_HOME")
	if adbPath != "" {
		return adbPath + "/platform-tools/adb"
	}

	// try default Android SDK location on macOS
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		defaultPath := filepath.Join(homeDir, "Library", "Android", "sdk", "platform-tools", "adb")
		if _, err := os.Stat(defaultPath); err == nil {
			return defaultPath
		}
	}

	// best effort, look in path
	return "adb"
}

func (d AndroidDevice) runAdbCommand(args ...string) ([]byte, error) {
	cmdArgs := append([]string{"-s", d.id}, args...)
	cmd := exec.Command(getAdbPath(), cmdArgs...)
	return cmd.CombinedOutput()
}

func (d AndroidDevice) TakeScreenshot() ([]byte, error) {
	byteData, err := d.runAdbCommand("shell", "screencap", "-p")
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %v", err)
	}

	return byteData, nil
}

func (d AndroidDevice) LaunchApp(bundleID string) error {
	output, err := d.runAdbCommand("shell", "monkey", "-p", bundleID, "-c", "android.intent.category.LAUNCHER", "1")
	if err != nil {
		return fmt.Errorf("failed to launch app %s: %v\nOutput: %s", bundleID, err, string(output))
	}

	return nil
}

func (d AndroidDevice) TerminateApp(bundleID string) error {
	output, err := d.runAdbCommand("shell", "am", "force-stop", bundleID)
	if err != nil {
		return fmt.Errorf("failed to terminate app %s: %v\nOutput: %s", bundleID, err, string(output))
	}

	return nil
}

// Reboot reboots the Android device/emulator using `adb reboot`.
func (d AndroidDevice) Reboot() error {
	_, err := d.runAdbCommand("reboot")
	if err != nil {
		return err
	}

	return nil
}

// Tap simulates a tap at (x, y) on the Android device.
func (d AndroidDevice) Tap(x, y int) error {
	_, err := d.runAdbCommand("shell", "input", "tap", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
	if err != nil {
		return err
	}

	return nil
}

// LongPress simulates a long press at (x, y) on the Android device.
func (d AndroidDevice) LongPress(x, y int) error {
	_, err := d.runAdbCommand("shell", "input", "swipe", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y), fmt.Sprintf("%d", x), fmt.Sprintf("%d", y), "500")
	if err != nil {
		return err
	}

	return nil
}

// Gesture performs a sequence of touch actions on the Android device
func (d AndroidDevice) Gesture(actions []wda.TapAction) error {

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
			deviceID := parts[0]
			status := parts[1]
			if status == "device" {
				devices = append(devices, AndroidDevice{
					id:      deviceID,
					name:    getAndroidDeviceName(deviceID),
					version: getAndroidDeviceVersion(deviceID),
				})
			}
		}
	}

	return devices
}

func getAndroidDeviceName(deviceID string) string {
	// Try getting AVD name first (for emulators)
	avdCmd := exec.Command(getAdbPath(), "-s", deviceID, "shell", "getprop", "ro.boot.qemu.avd_name")
	avdOutput, err := avdCmd.CombinedOutput()
	if err == nil && len(avdOutput) > 0 {
		avdName := strings.TrimSpace(string(avdOutput))
		if avdName != "" {
			return strings.ReplaceAll(avdName, "_", " ")
		}
	}

	// Fall back to product model
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
func GetAndroidDevices() ([]ControllableDevice, error) { // Changed return type
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

func (d AndroidDevice) StartAgent() error {
	// android doesn't need an agent to be started
	return nil
}

func (d AndroidDevice) PressButton(key string) error {
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

func (d AndroidDevice) SendKeys(text string) error {
	switch text {
	case "\b":
		return d.PressButton("BACKSPACE")
	case "\n":
		return d.PressButton("ENTER")
	}

	text = strings.ReplaceAll(text, " ", "\\ ")
	_, err := d.runAdbCommand("shell", "input", "text", text)
	return err
}

func (d AndroidDevice) OpenURL(url string) error {
	output, err := d.runAdbCommand("shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", url)
	if err != nil {
		return fmt.Errorf("failed to open URL %s: %v\nOutput: %s", url, err, string(output))
	}

	return nil
}

func (d AndroidDevice) ListApps() ([]InstalledAppInfo, error) {
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

func (d AndroidDevice) Info() (*FullDeviceInfo, error) {

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
		},
		ScreenSize: &ScreenSize{
			Width:  widthInt,
			Height: heightInt,
			Scale:  1,
		},
	}, nil
}

func (d AndroidDevice) GetAppPath(packageName string) (string, error) {
	output, err := d.runAdbCommand("shell", "pm", "path", packageName)
	if err != nil {
		// best effort (pm path will return error code 1)
		return "", nil
	}

	// remove the "package:" prefix
	appPath := strings.TrimPrefix(string(output), "package:")
	appPath = strings.TrimSuffix(appPath, "\n")
	return appPath, nil
}

func (d AndroidDevice) StartScreenCapture(format string, quality int, scale float64, callback func([]byte) bool) error {
	if format != "mjpeg" {
		return fmt.Errorf("unsupported format: %s, only 'mjpeg' is supported", format)
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

	utils.Verbose("Starting MJPEG server with app path: %s", appPath)
	cmdArgs := append([]string{"-s", d.id}, "shell", fmt.Sprintf("CLASSPATH=%s", appPath), "app_process", "/system/bin", "com.mobilenext.devicekit.MjpegServer", "--quality", fmt.Sprintf("%d", quality), "--scale", fmt.Sprintf("%.2f", scale))
	cmd := exec.Command(getAdbPath(), cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MJPEG server: %v", err)
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
			if !callback(buffer[:n]) {
				break
			}
		}
	}

	_ = cmd.Process.Kill()
	return nil
}

func (d AndroidDevice) installPackage(apkPath string) error {
	output, err := d.runAdbCommand("install", apkPath)
	if err != nil {
		return fmt.Errorf("failed to install package: %v\nOutput: %s", err, string(output))
	}

	if strings.Contains(string(output), "Success") {
		return nil
	}

	return fmt.Errorf("installation failed: %s", string(output))
}

func (d AndroidDevice) EnsureDeviceKitInstalled() error {
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
	XMLName     xml.Name               `xml:"node"`
	Class       string                 `xml:"class,attr"`
	Text        string                 `xml:"text,attr"`
	Bounds      string                 `xml:"bounds,attr"`
	Hint        string                 `xml:"hint,attr"`
	Focused     string                 `xml:"focused,attr"`
	ContentDesc string                 `xml:"content-desc,attr"`
	ResourceID  string                 `xml:"resource-id,attr"`
	Nodes       []uiAutomatorXmlNode   `xml:"node"`
}

type uiAutomatorXml struct {
	XMLName   xml.Name `xml:"hierarchy"`
	RootNode  uiAutomatorXmlNode `xml:"node"`
}

func (d AndroidDevice) getScreenElementRect(bounds string) types.ScreenElementRect {
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

func (d AndroidDevice) collectElements(node uiAutomatorXmlNode) []types.ScreenElement {
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

func (d AndroidDevice) getUiAutomatorDump() (string, error) {
	for tries := 0; tries < 10; tries++ {
		output, err := d.runAdbCommand("exec-out", "uiautomator", "dump", "/dev/tty")
		if err != nil {
			return "", fmt.Errorf("failed to run uiautomator dump: %v", err)
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

func (d AndroidDevice) DumpSource() ([]ScreenElement, error) {
	// get the XML dump from uiautomator
	xmlContent, err := d.getUiAutomatorDump()
	if err != nil {
		return nil, fmt.Errorf("failed to get uiautomator dump: %w", err)
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

func (d AndroidDevice) InstallApp(path string) error {
	output, err := d.runAdbCommand("install", "-r", path)
	if err != nil {
		return fmt.Errorf("failed to install app: %v\nOutput: %s", err, string(output))
	}

	if strings.Contains(string(output), "Success") {
		return nil
	}

	return fmt.Errorf("installation failed: %s", string(output))
}

func (d AndroidDevice) UninstallApp(packageName string) (*InstalledAppInfo, error) {
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
func (d AndroidDevice) GetOrientation() (string, error) {
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
func (d AndroidDevice) SetOrientation(orientation string) error {
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
