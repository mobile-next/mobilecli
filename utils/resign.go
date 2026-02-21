package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"howett.net/plist"
)

type provisioningProfile struct {
	Name              string        `plist:"Name"`
	TeamIdentifier    []string      `plist:"TeamIdentifier"`
	ProvisionedDevices []string     `plist:"ProvisionedDevices"`
	Entitlements      entitlements  `plist:"Entitlements"`
	ExpirationDate    time.Time     `plist:"ExpirationDate"`
}

type entitlements struct {
	GetTaskAllow          bool   `plist:"get-task-allow"`
	ApplicationIdentifier string `plist:"application-identifier"`
}

// ResignIPA re-signs an IPA file so it can be installed on a specific device.
// it returns the path to a new temporary IPA file that should be cleaned up by the caller.
func ResignIPA(ipaPath, deviceUDID, profileOverride, identityOverride string) (string, error) {
	// unzip IPA to temp dir
	tempDir, err := os.MkdirTemp("", "resign_")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	Verbose("Unzipping IPA to %s", tempDir)
	err = unzipForResign(ipaPath, tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to unzip IPA: %w", err)
	}

	// find the .app bundle inside Payload/
	appPath, err := findAppBundle(tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to find app bundle: %w", err)
	}

	Verbose("Found app bundle: %s", appPath)

	// read the app's bundle ID for profile matching
	bundleID, err := readBundleID(appPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to read bundle ID from app: %w", err)
	}

	Verbose("App bundle ID: %s", bundleID)

	// find provisioning profile
	profilePath, err := resolveProvisioningProfile(profileOverride, deviceUDID, bundleID)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", err
	}

	Verbose("Using provisioning profile: %s", profilePath)

	// extract team ID from profile
	teamID, err := extractTeamID(profilePath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to extract team ID from profile: %w", err)
	}

	Verbose("Team ID: %s", teamID)

	// find signing identity
	identity, err := resolveSigningIdentity(identityOverride, teamID)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", err
	}

	Verbose("Using signing identity: %s", identity)

	// extract entitlements from profile
	entitlementsPath, err := extractEntitlements(profilePath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to extract entitlements: %w", err)
	}
	defer func() { _ = os.Remove(entitlementsPath) }()

	Verbose("Extracted entitlements to %s", entitlementsPath)

	// copy profile into .app/embedded.mobileprovision
	embeddedPath := filepath.Join(appPath, "embedded.mobileprovision")
	err = CopyFile(profilePath, embeddedPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to copy provisioning profile: %w", err)
	}

	// re-sign embedded frameworks
	frameworksDir := filepath.Join(appPath, "Frameworks")
	if entries, err := os.ReadDir(frameworksDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".framework") || strings.HasSuffix(entry.Name(), ".dylib") {
				frameworkPath := filepath.Join(frameworksDir, entry.Name())
				Verbose("Signing framework: %s", entry.Name())
				err = codesign(frameworkPath, identity, "")
				if err != nil {
					_ = os.RemoveAll(tempDir)
					return "", fmt.Errorf("failed to sign framework %s: %w", entry.Name(), err)
				}
			}
		}
	}

	// re-sign app extensions
	pluginsDir := filepath.Join(appPath, "PlugIns")
	if entries, err := os.ReadDir(pluginsDir); err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".appex") {
				appexPath := filepath.Join(pluginsDir, entry.Name())
				Verbose("Signing app extension: %s", entry.Name())
				err = codesign(appexPath, identity, entitlementsPath)
				if err != nil {
					_ = os.RemoveAll(tempDir)
					return "", fmt.Errorf("failed to sign app extension %s: %w", entry.Name(), err)
				}
			}
		}
	}

	// re-sign the main .app bundle
	Verbose("Signing main app bundle")
	err = codesign(appPath, identity, entitlementsPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to sign app bundle: %w", err)
	}

	// re-zip into a new IPA
	outputIPA, err := os.CreateTemp("", "resigned_*.ipa")
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to create temp IPA file: %w", err)
	}
	outputPath := outputIPA.Name()
	_ = outputIPA.Close()

	Verbose("Repackaging IPA to %s", outputPath)
	cmd := exec.Command("zip", "-qr", outputPath, "Payload")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(tempDir)
		_ = os.Remove(outputPath)
		return "", fmt.Errorf("failed to repackage IPA: %w\n%s", err, output)
	}

	_ = os.RemoveAll(tempDir)
	return outputPath, nil
}

func unzipForResign(zipPath, destDir string) error {
	cmd := exec.Command("unzip", "-q", "-o", zipPath, "-d", destDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unzip failed: %w\n%s", err, output)
	}
	return nil
}

func findAppBundle(tempDir string) (string, error) {
	payloadDir := filepath.Join(tempDir, "Payload")
	entries, err := os.ReadDir(payloadDir)
	if err != nil {
		return "", fmt.Errorf("failed to read Payload directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") {
			return filepath.Join(payloadDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no .app bundle found in Payload/")
}

func resolveProvisioningProfile(profileOverride, deviceUDID, bundleID string) (string, error) {
	if profileOverride != "" {
		if _, err := os.Stat(profileOverride); err != nil {
			return "", fmt.Errorf("provisioning profile not found: %s", profileOverride)
		}
		return profileOverride, nil
	}
	return findMatchingProfile(deviceUDID, bundleID)
}

func resolveSigningIdentity(identityOverride, teamID string) (string, error) {
	if identityOverride != "" {
		return identityOverride, nil
	}
	return findSigningIdentity(teamID)
}

func readBundleID(appPath string) (string, error) {
	infoPlistPath := filepath.Join(appPath, "Info.plist")
	data, err := os.ReadFile(infoPlistPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Info.plist: %w", err)
	}

	var info map[string]interface{}
	_, err = plist.Unmarshal(data, &info)
	if err != nil {
		return "", fmt.Errorf("failed to parse Info.plist: %w", err)
	}

	bundleID, ok := info["CFBundleIdentifier"].(string)
	if !ok || bundleID == "" {
		return "", fmt.Errorf("CFBundleIdentifier not found in Info.plist")
	}

	return bundleID, nil
}

func decodeProvisioningProfile(profilePath string) (*provisioningProfile, error) {
	cmd := exec.Command("security", "cms", "-D", "-i", profilePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to decode profile %s: %w", profilePath, err)
	}

	var profile provisioningProfile
	_, err = plist.Unmarshal(output, &profile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profile plist: %w", err)
	}

	return &profile, nil
}

func findMatchingProfile(deviceUDID, bundleID string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	profilesDir := filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles")
	// also check the Xcode managed profiles directory
	xcodeProfilesDir := filepath.Join(homeDir, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles")

	var searchDirs []string
	for _, dir := range []string{profilesDir, xcodeProfilesDir} {
		if _, err := os.Stat(dir); err == nil {
			searchDirs = append(searchDirs, dir)
		}
	}

	if len(searchDirs) == 0 {
		return "", noMatchingProfileError(deviceUDID, bundleID)
	}

	// prefer exact bundle ID match over wildcard
	var wildcardMatch string

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".mobileprovision") {
				continue
			}

			fullPath := filepath.Join(dir, entry.Name())
			profile, err := decodeProvisioningProfile(fullPath)
			if err != nil {
				Verbose("Skipping profile %s: %v", entry.Name(), err)
				continue
			}

			// check not expired
			if profile.ExpirationDate.Before(time.Now()) {
				Verbose("Skipping expired profile: %s", profile.Name)
				continue
			}

			// check it's a development profile (get-task-allow: true)
			if !profile.Entitlements.GetTaskAllow {
				Verbose("Skipping non-development profile: %s", profile.Name)
				continue
			}

			// check it contains the device UDID
			hasDevice := false
			for _, udid := range profile.ProvisionedDevices {
				if udid == deviceUDID {
					hasDevice = true
					break
				}
			}
			if !hasDevice {
				Verbose("Skipping profile %s: device %s not included", profile.Name, deviceUDID)
				continue
			}

			appID := profile.Entitlements.ApplicationIdentifier
			if len(profile.TeamIdentifier) == 0 {
				continue
			}

			teamPrefix := profile.TeamIdentifier[0] + "."

			// exact bundle ID match (e.g. "TEAMID.com.example.app")
			if appID == teamPrefix+bundleID {
				Verbose("Found exact-match profile: %s (%s)", profile.Name, fullPath)
				return fullPath, nil
			}

			// wildcard match (e.g. "TEAMID.*")
			if appID == teamPrefix+"*" && wildcardMatch == "" {
				Verbose("Found wildcard profile: %s (%s)", profile.Name, fullPath)
				wildcardMatch = fullPath
			}
		}
	}

	if wildcardMatch != "" {
		return wildcardMatch, nil
	}

	return "", noMatchingProfileError(deviceUDID, bundleID)
}

func noMatchingProfileError(deviceUDID, bundleID string) error {
	return fmt.Errorf(`no provisioning profile found matching bundle ID %s for device %s. to fix this, either:
a) build this app for your device in Xcode (creates a matching profile automatically)
b) create a wildcard provisioning profile:
   1. in Apple Developer portal (developer.apple.com/account), create a wildcard App ID (bundle ID "*")
   2. create a new iOS Development provisioning profile using the wildcard App ID, including your device
   3. download and double-click the profile to install it
c) use --provisioning-profile to specify a profile manually
note: provisioning profiles require a paid Apple Developer Program membership ($99/year)`, bundleID, deviceUDID)
}

func findSigningIdentity(teamID string) (string, error) {
	cmd := exec.Command("security", "find-identity", "-v", "-p", "codesigning")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list signing identities: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// look for lines like: 1) HASH "Apple Development: Name (TEAMID)"
		if !strings.Contains(line, "Apple Development") && !strings.Contains(line, "iPhone Developer") {
			continue
		}

		// extract the identity string between quotes
		startQuote := strings.Index(line, "\"")
		endQuote := strings.LastIndex(line, "\"")
		if startQuote == -1 || endQuote == -1 || startQuote == endQuote {
			continue
		}

		identity := line[startQuote+1 : endQuote]

		// check if the identity contains the team ID
		if strings.Contains(identity, teamID) {
			return identity, nil
		}
	}

	// if no team-specific match, try any valid development identity
	for _, line := range lines {
		if !strings.Contains(line, "Apple Development") && !strings.Contains(line, "iPhone Developer") {
			continue
		}

		startQuote := strings.Index(line, "\"")
		endQuote := strings.LastIndex(line, "\"")
		if startQuote == -1 || endQuote == -1 || startQuote == endQuote {
			continue
		}

		identity := line[startQuote+1 : endQuote]
		Verbose("Using signing identity (no team match): %s", identity)
		return identity, nil
	}

	return "", fmt.Errorf("no Apple Development signing identity found in keychain")
}

func extractEntitlements(profilePath string) (string, error) {
	profile, err := decodeProvisioningProfile(profilePath)
	if err != nil {
		return "", err
	}

	// build the entitlements plist with the values from the profile
	entitlementsMap := map[string]interface{}{
		"application-identifier": profile.Entitlements.ApplicationIdentifier,
		"get-task-allow":         profile.Entitlements.GetTaskAllow,
	}

	// add team identifier if available
	if len(profile.TeamIdentifier) > 0 {
		entitlementsMap["com.apple.developer.team-identifier"] = profile.TeamIdentifier[0]
	}

	plistData, err := plist.MarshalIndent(entitlementsMap, plist.XMLFormat, "\t")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entitlements plist: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "entitlements_*.plist")
	if err != nil {
		return "", fmt.Errorf("failed to create temp entitlements file: %w", err)
	}

	_, err = tmpFile.Write(plistData)
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write entitlements: %w", err)
	}

	_ = tmpFile.Close()
	return tmpFile.Name(), nil
}

func extractTeamID(profilePath string) (string, error) {
	profile, err := decodeProvisioningProfile(profilePath)
	if err != nil {
		return "", err
	}

	if len(profile.TeamIdentifier) == 0 {
		return "", fmt.Errorf("no team identifier found in profile")
	}

	return profile.TeamIdentifier[0], nil
}

func codesign(path, identity, entitlementsPath string) error {
	args := []string{"--force", "--sign", identity, "--timestamp=none", "--generate-entitlement-der"}
	if entitlementsPath != "" {
		args = append(args, "--entitlements", entitlementsPath)
	}
	args = append(args, path)

	cmd := exec.Command("codesign", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign failed: %w\n%s", err, output)
	}
	return nil
}
