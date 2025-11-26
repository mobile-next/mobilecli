package devices

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mobile-next/mobilecli/utils"
	"gopkg.in/ini.v1"
)

// AVDInfo represents information about an Android Virtual Device
type AVDInfo struct {
	Name     string
	Device   string
	APILevel string
}

// apiLevelToVersion maps Android API levels to version strings
var apiLevelToVersion = map[string]string{
	"36": "16.0",
	"35": "15.0",
	"34": "14.0",
	"33": "13.0",
	"32": "12.1", // Android 12L
	"31": "12.0",
	"30": "11.0",
	"29": "10.0",
	"28": "9.0",
	"27": "8.1",
	"26": "8.0",
	"25": "7.1",
	"24": "7.0",
	"23": "6.0",
	"22": "5.1",
	"21": "5.0",
}

// convertAPILevelToVersion converts an API level to Android version string
func convertAPILevelToVersion(apiLevel string) string {
	if version, ok := apiLevelToVersion[apiLevel]; ok {
		return version
	}
	// if no mapping found, return the API level as-is
	return apiLevel
}

// listAllAVDs retrieves all available AVDs using emulator -list-avds
func listAllAVDs() ([]string, error) {
	cmd := exec.Command(getEmulatorPath(), "-list-avds")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// if emulator command fails, return empty list (SDK might not be installed)
		utils.Verbose("Failed to list AVDs: %v", err)
		return []string{}, nil
	}

	var avdNames []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			avdNames = append(avdNames, line)
		}
	}

	return avdNames, nil
}

// getAVDDetails retrieves AVD information by reading .ini files directly
func getAVDDetails() (map[string]AVDInfo, error) {
	avdMap := make(map[string]AVDInfo)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return avdMap, err
	}

	avdDir := filepath.Join(homeDir, ".android", "avd")
	pattern := filepath.Join(avdDir, "*.ini")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return avdMap, err
	}

	for _, iniFile := range matches {
		// read the .ini file to get the path
		iniConfig, err := ini.Load(iniFile)
		if err != nil {
			utils.Verbose("Failed to read %s: %v", iniFile, err)
			continue
		}

		avdPath := iniConfig.Section("").Key("path").String()
		if avdPath == "" {
			continue
		}

		// read the config.ini inside the .avd directory
		configPath := filepath.Join(avdPath, "config.ini")
		configData, err := ini.Load(configPath)
		if err != nil {
			utils.Verbose("Failed to read %s: %v", configPath, err)
			continue
		}

		displayName := configData.Section("").Key("avd.ini.displayname").String()
		if displayName == "" {
			continue
		}

		// extract API level from target (e.g., "android-31" -> "31")
		target := configData.Section("").Key("target").String()
		apiLevel := strings.TrimPrefix(target, "android-")

		// extract avd name from .ini filename
		avdName := strings.TrimSuffix(filepath.Base(iniFile), ".ini")

		avdMap[avdName] = AVDInfo{
			Name:     displayName,
			Device:   displayName,
			APILevel: apiLevel,
		}
	}

	return avdMap, nil
}

// getOfflineAndroidEmulators returns a list of offline Android emulators (AVDs not currently running)
func getOfflineAndroidEmulators(onlineDeviceIDs map[string]bool) ([]ControllableDevice, error) {
	var offlineDevices []ControllableDevice

	// get list of all AVDs
	avdNames, err := listAllAVDs()
	if err != nil {
		return offlineDevices, err
	}

	if len(avdNames) == 0 {
		return offlineDevices, nil
	}

	// get detailed info about AVDs
	avdDetails, err := getAVDDetails()
	if err != nil {
		return offlineDevices, err
	}

	// create offline device entries for AVDs that are not running
	for _, avdName := range avdNames {
		// check if this AVD is already online
		// online emulators have device name in getprop ro.boot.qemu.avd_name
		isOnline := false
		for deviceID := range onlineDeviceIDs {
			deviceName := getAndroidDeviceName(deviceID)
			if matchesAVDName(avdName, deviceName) {
				isOnline = true
				break
			}
		}

		if !isOnline {
			info, hasDetails := avdDetails[avdName]
			displayName := strings.ReplaceAll(avdName, "_", " ")
			version := ""

			if hasDetails {
				if info.Device != "" {
					// use device name if available (e.g., "pixel_6 (Google)")
					displayName = info.Device
					// clean up the device name
					if idx := strings.Index(displayName, "("); idx > 0 {
						displayName = strings.TrimSpace(displayName[:idx])
					}
					displayName = strings.ReplaceAll(displayName, "_", " ")
				}
				version = convertAPILevelToVersion(info.APILevel)
			}

			offlineDevices = append(offlineDevices, &AndroidDevice{
				id:      avdName,
				name:    displayName,
				version: version,
				state:   "offline",
			})
		}
	}

	return offlineDevices, nil
}
