package devices

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/mobile-next/mobilecli/utils"
)

// AVDInfo represents information about an Android Virtual Device
type AVDInfo struct {
	Name     string
	Device   string
	Path     string
	Target   string
	BasedOn  string
	APILevel string
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

// parseAVDManagerOutput parses the output of 'avdmanager list avd' and extracts detailed info
func parseAVDManagerOutput(output string) map[string]AVDInfo {
	avdMap := make(map[string]AVDInfo)
	lines := strings.Split(output, "\n")

	var currentAVD AVDInfo
	var currentName string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Name:") {
			// save previous AVD if exists
			if currentName != "" {
				avdMap[currentName] = currentAVD
			}
			// start new AVD
			currentName = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			currentAVD = AVDInfo{Name: currentName}
		} else if strings.HasPrefix(line, "Device:") {
			currentAVD.Device = strings.TrimSpace(strings.TrimPrefix(line, "Device:"))
		} else if strings.HasPrefix(line, "Path:") {
			currentAVD.Path = strings.TrimSpace(strings.TrimPrefix(line, "Path:"))
		} else if strings.HasPrefix(line, "Target:") {
			currentAVD.Target = strings.TrimSpace(strings.TrimPrefix(line, "Target:"))
		} else if strings.HasPrefix(line, "Based on:") {
			basedOn := strings.TrimSpace(strings.TrimPrefix(line, "Based on:"))
			currentAVD.BasedOn = basedOn

			// extract API level from "Based on:" line
			// format: "Android 12.0 ("S") Tag/ABI: google_apis/arm64-v8a" or "Android API 36 Tag/ABI: ..."
			if strings.Contains(basedOn, "API") {
				re := regexp.MustCompile(`API\s+(\d+)`)
				matches := re.FindStringSubmatch(basedOn)
				if len(matches) > 1 {
					currentAVD.APILevel = matches[1]
				}
			} else {
				// try to extract version number for named versions like "12.0"
				re := regexp.MustCompile(`Android\s+(\d+\.\d+)`)
				matches := re.FindStringSubmatch(basedOn)
				if len(matches) > 1 {
					currentAVD.APILevel = matches[1]
				}
			}
		}
	}

	// save last AVD
	if currentName != "" {
		avdMap[currentName] = currentAVD
	}

	return avdMap
}

// getAVDDetails retrieves detailed information about all AVDs
func getAVDDetails() (map[string]AVDInfo, error) {
	cmd := exec.Command(getAvdManagerPath(), "list", "avd")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// if avdmanager fails, return empty map
		utils.Verbose("Failed to get AVD details: %v", err)
		return make(map[string]AVDInfo), nil
	}

	return parseAVDManagerOutput(string(output)), nil
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
				version = info.APILevel
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
