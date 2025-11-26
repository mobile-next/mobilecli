package devices

import (
	"os"
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
	AvdId    string
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
		// extract avd name from .ini filename
		avdName := strings.TrimSuffix(filepath.Base(iniFile), ".ini")

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

		// get AvdId for matching with online devices
		avdId := configData.Section("").Key("AvdId").String()

		avdMap[avdName] = AVDInfo{
			Name:     displayName,
			Device:   displayName,
			APILevel: apiLevel,
			AvdId:    avdId,
		}
	}

	return avdMap, nil
}

// getOfflineAndroidEmulators returns a list of offline Android emulators (AVDs not currently running)
func getOfflineAndroidEmulators(onlineDeviceIDs map[string]bool) ([]ControllableDevice, error) {
	var offlineDevices []ControllableDevice

	// get detailed info about AVDs by reading .ini files directly
	avdDetails, err := getAVDDetails()
	if err != nil {
		return offlineDevices, err
	}

	if len(avdDetails) == 0 {
		return offlineDevices, nil
	}

	// create offline device entries for AVDs that are not running
	for avdName, info := range avdDetails {
		// check if this AVD is already online by checking if AvdId is in online device IDs
		// the avdName from the .ini file should match the device ID when online
		_, isOnline := onlineDeviceIDs[info.AvdId]

		if !isOnline {
			displayName := strings.ReplaceAll(avdName, "_", " ")
			version := ""

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
