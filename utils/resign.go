package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	tempDir, err := os.MkdirTemp("", "resign_")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	Verbose("Unzipping IPA to %s", tempDir)
	if err = unzipFile(ipaPath, tempDir); err != nil {
		return "", fmt.Errorf("failed to unzip IPA: %w", err)
	}

	appPath, err := findAppBundle(tempDir)
	if err != nil {
		return "", fmt.Errorf("failed to find app bundle: %w", err)
	}
	Verbose("Found app bundle: %s", appPath)

	bundleID, err := readBundleID(appPath)
	if err != nil {
		return "", fmt.Errorf("failed to read bundle ID from app: %w", err)
	}
	Verbose("App bundle ID: %s", bundleID)

	profilePath, err := resolveProvisioningProfile(profileOverride, deviceUDID, bundleID)
	if err != nil {
		return "", err
	}
	Verbose("Using provisioning profile: %s", profilePath)

	profile, err := decodeProvisioningProfile(profilePath)
	if err != nil {
		return "", fmt.Errorf("failed to decode provisioning profile: %w", err)
	}

	if len(profile.TeamIdentifier) == 0 {
		return "", fmt.Errorf("no team identifier found in profile")
	}
	teamID := profile.TeamIdentifier[0]
	Verbose("Team ID: %s", teamID)

	identity, err := resolveSigningIdentity(identityOverride, teamID)
	if err != nil {
		return "", err
	}
	Verbose("Using signing identity: %s", identity)

	entitlementsPath, err := writeEntitlementsPlist(profile)
	if err != nil {
		return "", fmt.Errorf("failed to extract entitlements: %w", err)
	}
	defer func() { _ = os.Remove(entitlementsPath) }()
	Verbose("Extracted entitlements to %s", entitlementsPath)

	embeddedPath := filepath.Join(appPath, "embedded.mobileprovision")
	if err = CopyFile(profilePath, embeddedPath); err != nil {
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
					return "", fmt.Errorf("failed to sign app extension %s: %w", entry.Name(), err)
				}
			}
		}
	}

	Verbose("Signing main app bundle")
	if err = codesign(appPath, identity, entitlementsPath); err != nil {
		return "", fmt.Errorf("failed to sign app bundle: %w", err)
	}

	outputIPA, err := os.CreateTemp("", "resigned_*.ipa")
	if err != nil {
		return "", fmt.Errorf("failed to create temp IPA file: %w", err)
	}
	outputPath := outputIPA.Name()
	_ = outputIPA.Close()
	// remove the empty temp file so zip can create it fresh
	_ = os.Remove(outputPath)

	Verbose("Repackaging IPA to %s", outputPath)
	cmd := exec.Command("zip", "-qr", outputPath, "Payload")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(outputPath)
		return "", fmt.Errorf("failed to repackage IPA: %w\n%s", err, output)
	}

	return outputPath, nil
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

	var info map[string]any
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

type profileMatch int

const (
	profileMatchNone profileMatch = iota
	profileMatchWildcard
	profileMatchExact
)

func matchProfile(profile *provisioningProfile, deviceUDID, bundleID string) profileMatch {
	if profile.ExpirationDate.Before(time.Now()) {
		Verbose("Skipping expired profile: %s", profile.Name)
		return profileMatchNone
	}

	if !profile.Entitlements.GetTaskAllow {
		Verbose("Skipping non-development profile: %s", profile.Name)
		return profileMatchNone
	}

	if !slices.Contains(profile.ProvisionedDevices, deviceUDID) {
		Verbose("Skipping profile %s: device %s not included", profile.Name, deviceUDID)
		return profileMatchNone
	}

	if len(profile.TeamIdentifier) == 0 {
		return profileMatchNone
	}

	appID := profile.Entitlements.ApplicationIdentifier
	teamPrefix := profile.TeamIdentifier[0] + "."

	if appID == teamPrefix+bundleID {
		return profileMatchExact
	}

	if appID == teamPrefix+"*" {
		return profileMatchWildcard
	}

	return profileMatchNone
}

func findMatchingProfile(deviceUDID, bundleID string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	profilesDir := filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles")
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

			switch matchProfile(profile, deviceUDID, bundleID) {
			case profileMatchExact:
				Verbose("Found exact-match profile: %s (%s)", profile.Name, fullPath)
				return fullPath, nil
			case profileMatchWildcard:
				if wildcardMatch == "" {
					Verbose("Found wildcard profile: %s (%s)", profile.Name, fullPath)
					wildcardMatch = fullPath
				}
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

	// pre-fetch all certificates so OU lookups don't shell out per identity
	certDump := dumpAllCertificates()

	lines := strings.Split(string(output), "\n")
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

		// check display name first (e.g. "Apple Development: Name (TEAMID)").
		// return the SHA-1 hash to avoid ambiguity when multiple certs share the same name.
		if strings.Contains(identity, teamID) {
			hash := extractCertHash(line)
			if hash != "" {
				return hash, nil
			}
			return identity, nil
		}

		// the display name may contain a personal ID instead of the team ID,
		// so also check the certificate's OU field which holds the actual team ID.
		hash := extractCertHash(line)
		if hash != "" && certOUMatchesTeam(certDump, hash, teamID) {
			Verbose("Found signing identity via certificate OU: %s (hash: %s)", identity, hash)
			return hash, nil
		}
	}

	return "", fmt.Errorf("no Apple Development signing identity found for team %s. create one in Xcode: Settings → Accounts → select your account → Manage Certificates → + → Apple Development", teamID)
}

func extractCertHash(identityLine string) string {
	parts := strings.Fields(identityLine)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func dumpAllCertificates() string {
	cmd := exec.Command("security", "find-certificate", "-a", "-Z")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(output)
}

func certOUMatchesTeam(certDump, certHash, teamID string) bool {
	lines := strings.Split(certDump, "\n")
	var inTargetCert bool

	for _, line := range lines {
		if strings.HasPrefix(line, "SHA-1 hash:") {
			hash := strings.TrimSpace(strings.TrimPrefix(line, "SHA-1 hash:"))
			inTargetCert = (hash == certHash)
		}
		// the "subj" blob contains the certificate subject with OU
		if inTargetCert && strings.Contains(line, "\"subj\"") && strings.Contains(line, teamID) {
			return true
		}
	}

	return false
}

func writeEntitlementsPlist(profile *provisioningProfile) (string, error) {
	entitlementsMap := map[string]any{
		"application-identifier": profile.Entitlements.ApplicationIdentifier,
		"get-task-allow":         profile.Entitlements.GetTaskAllow,
	}

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
