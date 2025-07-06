package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/utils"
)

// AndroidDevice implements the ControllableDevice interface for Android devices
type AndroidDevice struct {
	id   string
	name string
}

func (d AndroidDevice) ID() string {
	return d.id
}

func (d AndroidDevice) Name() string {
	return d.name
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
					id:   deviceID,
					name: getAndroidDeviceName(deviceID),
				})
			}
		}
	}

	return devices
}

func getAndroidDeviceName(deviceID string) string {
	modelCmd := exec.Command(getAdbPath(), "-s", deviceID, "shell", "getprop", "ro.product.model")
	modelOutput, err := modelCmd.CombinedOutput()
	if err == nil && len(modelOutput) > 0 {
		return strings.TrimSpace(string(modelOutput))
	}

	return deviceID
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
	if text == "\b" {
		return d.PressButton("BACKSPACE")
	} else if text == "\n" {
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
		},
		ScreenSize: &ScreenSize{
			Width:  widthInt,
			Height: heightInt,
			Scale:  1,
		},
	}, nil
}

func (d AndroidDevice) ConvertPNGtoJPEG(pngData []byte, quality int) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %v", err)
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %v", err)
	}

	return buf.Bytes(), nil
}

func (d AndroidDevice) StartScreenCapture(format string, callback func([]byte) bool) error {
	if format != "mjpeg" {
		return fmt.Errorf("unsupported format: %s, only 'mjpeg' is supported", format)
	}

	err := d.EnsureDeviceKitInstalled()
	if err != nil {
		return fmt.Errorf("failed to get app path: %v", err)
	}

	cmdArgs := append([]string{"-s", d.id}, "shell", fmt.Sprintf("CLASSPATH=%s", "x"), "app_process", "/system/bin", "com.mobilenext.devicekit.MjpegServer")
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

	cmd.Process.Kill()
	return nil
}

type githubRelease struct {
	Assets []struct {
		BrowserDownloadURL string `json:"browser_download_url"`
		Name               string `json:"name"`
	} `json:"assets"`
}

func (d AndroidDevice) isPackageInstalled(packageName string) (bool, error) {
	output, err := d.runAdbCommand("shell", "pm", "list", "packages", packageName)
	if err != nil {
		return false, fmt.Errorf("failed to check package installation: %v", err)
	}

	return strings.Contains(string(output), packageName), nil
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

	installed, err := d.isPackageInstalled(packageName)
	if err != nil {
		return fmt.Errorf("failed to check if %s is installed: %v", packageName, err)
	}

	if installed {
		return nil
	}

	utils.Verbose("DeviceKit not installed, downloading and installing...")

	resp, err := http.Get("https://api.github.com/repos/mobile-next/devicekit-android/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to decode release JSON: %v", err)
	}

	if len(release.Assets) == 0 {
		return fmt.Errorf("no assets found in latest release")
	}

	downloadURL := release.Assets[0].BrowserDownloadURL
	utils.Verbose("Downloading APK from: %s", downloadURL)

	tempDir, err := os.MkdirTemp("", "devicekit-android-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	apkPath := filepath.Join(tempDir, "devicekit.apk")

	apkResp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download APK: %v", err)
	}
	defer apkResp.Body.Close()

	if apkResp.StatusCode != http.StatusOK {
		return fmt.Errorf("APK download returned status %d", apkResp.StatusCode)
	}

	apkFile, err := os.Create(apkPath)
	if err != nil {
		return fmt.Errorf("failed to create APK file: %v", err)
	}
	defer apkFile.Close()

	if _, err := io.Copy(apkFile, apkResp.Body); err != nil {
		return fmt.Errorf("failed to write APK file: %v", err)
	}

	utils.Verbose("Installing APK...")
	if err := d.installPackage(apkPath); err != nil {
		return fmt.Errorf("failed to install APK: %v", err)
	}

	installed, err = d.isPackageInstalled(packageName)
	if err != nil {
		return fmt.Errorf("failed to verify installation: %v", err)
	}

	if !installed {
		return fmt.Errorf("package %s was not installed successfully", packageName)
	}

	utils.Verbose("DeviceKit successfully installed")
	return nil
}
