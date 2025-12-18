package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type DoctorInfo struct {
	MobileCLIVersion        string `json:"mobilecli_version"`
	OS                      string `json:"os"`
	OSVersion               string `json:"os_version"`
	AndroidHome             string `json:"android_home"`
	ADBPath                 string `json:"adb_path"`
	ADBVersion              string `json:"adb_version,omitempty"`
	EmulatorPath            string `json:"emulator_path"`
	XcodePath               string `json:"xcode_path,omitempty"`
	XcodeCLIToolsPath       string `json:"xcode_cli_tools_path,omitempty"`
	DevToolsSecurityEnabled *bool  `json:"devtools_security_enabled,omitempty"`
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

		if _, err := os.Stat(adbPath); err == nil {
			return adbPath
		}
	}

	// check if adb is in PATH
	adbPath, err := exec.LookPath("adb")
	if err == nil {
		return adbPath
	}

	return ""
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

	// check if emulator is in PATH
	emulatorPath, err := exec.LookPath("emulator")
	if err == nil {
		return emulatorPath
	}

	return ""
}

func getAdbVersion(adbPath string) string {
	if adbPath == "" {
		return ""
	}

	cmd := exec.Command(adbPath, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// parse the output to get just the version line
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Android Debug Bridge version") {
			return strings.TrimSpace(line)
		}
	}

	return strings.TrimSpace(string(output))
}

func getXcodePath() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	// check if Xcode.app is installed
	cmd := exec.Command("xcode-select", "-p")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	path := strings.TrimSpace(string(output))

	// check if this is the full Xcode.app path
	if strings.Contains(path, "Xcode.app") {
		return path
	}

	return ""
}

func getXcodeCLIToolsPath() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	cmd := exec.Command("xcode-select", "-p")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	path := strings.TrimSpace(string(output))

	// verify the path exists
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}

func getDevToolsSecurityEnabled() *bool {
	if runtime.GOOS != "darwin" {
		return nil
	}

	cmd := exec.Command("DevToolsSecurity", "-status")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return nil
	}

	outputStr := strings.TrimSpace(string(output))
	// the output is typically "Developer mode is currently enabled." or "Developer mode is currently disabled."
	enabled := strings.Contains(strings.ToLower(outputStr), "enabled")

	return &enabled
}

func getOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("sw_vers", "-productVersion")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(output))
	case "windows":
		cmd := exec.Command("cmd", "/c", "ver")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(output))
	case "linux":
		// try reading /etc/os-release
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return ""
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			}
		}
		return ""
	default:
		return ""
	}
}

// DoctorCommand performs system diagnostics and returns information about the environment
func DoctorCommand(version string) *CommandResponse {
	info := DoctorInfo{
		MobileCLIVersion: version,
		OS:               runtime.GOOS,
		OSVersion:        getOSVersion(),
		AndroidHome:      os.Getenv("ANDROID_HOME"),
		ADBPath:          getAdbPath(),
		EmulatorPath:     getEmulatorPath(),
	}

	// get adb version if adb is available
	if info.ADBPath != "" {
		info.ADBVersion = getAdbVersion(info.ADBPath)
	}

	// only get Xcode path on darwin
	if runtime.GOOS == "darwin" {
		info.XcodePath = getXcodePath()
		info.XcodeCLIToolsPath = getXcodeCLIToolsPath()
		info.DevToolsSecurityEnabled = getDevToolsSecurityEnabled()
	}

	return NewSuccessResponse(info)
}
