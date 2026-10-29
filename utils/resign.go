package utils

import (
	"context"
	"crypto/sha1" // #nosec G505 -- Apple identifies signing identities by the certificate's SHA-1 fingerprint.
	"encoding/hex"
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
	Name                  string       `plist:"Name"`
	TeamIdentifier        []string     `plist:"TeamIdentifier"`
	ProvisionedDevices    []string     `plist:"ProvisionedDevices"`
	DeveloperCertificates [][]byte     `plist:"DeveloperCertificates"`
	Entitlements          entitlements `plist:"Entitlements"`
	ExpirationDate        time.Time    `plist:"ExpirationDate"`
}

type entitlements struct {
	GetTaskAllow          bool   `plist:"get-task-allow"`
	ApplicationIdentifier string `plist:"application-identifier"`
}

// ResignOptions controls optional changes made while re-signing an IPA.
type ResignOptions struct {
	DeriveBundleIDFromProfile bool
}

const agentRunnerBundleID = "com.mobilenext.devicekit-iosUITests.xctrunner"

// ResignIPA re-signs an IPA file so it can be installed on a specific device.
// it returns the path to a new temporary IPA file that should be cleaned up by the caller.
func ResignIPA(ipaPath, deviceUDID, profileOverride, identityOverride string, opts ResignOptions) (string, error) {
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

	if len(profile.TeamIdentifier) == 0 || profile.TeamIdentifier[0] == "" {
		return "", fmt.Errorf("no team identifier found in profile")
	}
	teamID := profile.TeamIdentifier[0]
	Verbose("Team ID: %s", teamID)

	if err := configureBundleID(appPath, profile, bundleID, opts); err != nil {
		return "", err
	}

	identity, err := resolveSigningIdentity(identityOverride, profile)
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

	if err = signAppBundle(appPath, identity, entitlementsPath); err != nil {
		return "", err
	}

	return repackageIPA(tempDir)
}

// signAppBundle signs inside out: frameworks, then plugins, then the app itself.
func signAppBundle(appPath, identity, entitlementsPath string) error {
	if err := signFrameworks(appPath, identity); err != nil {
		return err
	}
	if err := signPlugins(appPath, identity, entitlementsPath); err != nil {
		return err
	}
	if err := signLooseDylibs(appPath, identity); err != nil {
		return err
	}

	Verbose("Signing main app bundle")
	if err := codesign(appPath, identity, entitlementsPath); err != nil {
		return fmt.Errorf("failed to sign app bundle: %w", err)
	}
	return nil
}

// signFrameworks re-signs embedded frameworks; an app without a Frameworks dir has none.
func signFrameworks(appPath, identity string) error {
	frameworksDir := filepath.Join(appPath, "Frameworks")
	entries, err := os.ReadDir(frameworksDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read Frameworks directory: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".framework") && !strings.HasSuffix(entry.Name(), ".dylib") {
			continue
		}
		Verbose("Signing framework: %s", entry.Name())
		if err := codesign(filepath.Join(frameworksDir, entry.Name()), identity, ""); err != nil {
			return fmt.Errorf("failed to sign framework %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// signPlugins re-signs app extensions and xctest bundles; an app without a PlugIns dir has none.
func signPlugins(appPath, identity, entitlementsPath string) error {
	pluginsDir := filepath.Join(appPath, "PlugIns")
	entries, err := os.ReadDir(pluginsDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read PlugIns directory: %w", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".appex") && !strings.HasSuffix(entry.Name(), ".xctest") {
			continue
		}
		pluginPath := filepath.Join(pluginsDir, entry.Name())
		if err := signLooseDylibs(pluginPath, identity); err != nil {
			return err
		}
		Verbose("Signing plugin: %s", entry.Name())
		if err := codesign(pluginPath, identity, entitlementsPath); err != nil {
			return fmt.Errorf("failed to sign plugin %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// repackageIPA zips the Payload dir under tempDir into a new temp IPA and returns its path.
func repackageIPA(tempDir string) (string, error) {
	outputIPA, err := os.CreateTemp("", "resigned_*.ipa")
	if err != nil {
		return "", fmt.Errorf("failed to create temp IPA file: %w", err)
	}
	outputPath := outputIPA.Name()
	_ = outputIPA.Close()
	// remove the empty temp file so zip can create it fresh
	_ = os.Remove(outputPath)

	Verbose("Repackaging IPA to %s", outputPath)
	cmd := exec.CommandContext(context.Background(), "zip", "-qr", outputPath, "Payload")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(outputPath)
		return "", fmt.Errorf("failed to repackage IPA: %w\n%s", err, output)
	}

	return outputPath, nil
}

// looseDylibs lists the dylibs sitting directly in a bundle's root. Xcode debug
// builds put the real code there (<Name>.debug.dylib next to a stub executable,
// plus __preview.dylib). signing the bundle does not sign them, and dyld refuses
// an unsigned one at launch: "mapped file has no cdhash, completely unsigned".
func looseDylibs(bundlePath string) ([]string, error) {
	entries, err := os.ReadDir(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle %s: %w", bundlePath, err)
	}
	var dylibs []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".dylib") {
			dylibs = append(dylibs, filepath.Join(bundlePath, entry.Name()))
		}
	}
	return dylibs, nil
}

// signLooseDylibs must run before the bundle itself is signed: code is signed
// inside-out, and the bundle's seal covers these files.
func signLooseDylibs(bundlePath, identity string) error {
	dylibs, err := looseDylibs(bundlePath)
	if err != nil {
		return err
	}
	for _, dylib := range dylibs {
		Verbose("Signing dylib: %s", filepath.Base(dylib))
		if err := codesign(dylib, identity, ""); err != nil {
			return fmt.Errorf("failed to sign dylib %s: %w", filepath.Base(dylib), err)
		}
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

func resolveSigningIdentity(identityOverride string, profile *provisioningProfile) (string, error) {
	if identityOverride != "" {
		return identityOverride, nil
	}
	return findSigningIdentity(profile)
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
	cmd := exec.CommandContext(context.Background(), "openssl", "smime", "-inform", "DER", "-verify", "-noverify", "-in", profilePath)
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

func findSigningIdentity(profile *provisioningProfile) (string, error) {
	cmd := exec.CommandContext(context.Background(), "/usr/bin/security", "find-identity", "-v", "-p", "codesigning")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list signing identities: %w", err)
	}

	identity := findProfileSigningIdentity(string(output), profile.DeveloperCertificates)
	if identity != "" {
		Verbose("Found signing identity included in provisioning profile: %s", identity)
		return identity, nil
	}

	return "", fmt.Errorf("none of the signing certificates included in provisioning profile %q have a corresponding private key in the keychain", profile.Name)
}

func findProfileSigningIdentity(identityOutput string, developerCertificates [][]byte) string {
	profileHashes := make(map[string]struct{}, len(developerCertificates))
	for _, certificate := range developerCertificates {
		// #nosec G401 -- Apple identifies signing identities by the certificate's SHA-1 fingerprint.
		sum := sha1.Sum(certificate)
		profileHashes[strings.ToUpper(hex.EncodeToString(sum[:]))] = struct{}{}
	}

	for _, line := range strings.Split(identityOutput, "\n") {
		hash := strings.ToUpper(extractCertHash(line))
		if _, matchesProfile := profileHashes[hash]; matchesProfile {
			return hash
		}
	}

	return ""
}

func extractCertHash(identityLine string) string {
	parts := strings.Fields(identityLine)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
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

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write entitlements: %w", err)
	}
	return tmpFile.Name(), nil
}

func codesign(path, identity, entitlementsPath string) error {
	args := []string{"--force", "--sign", identity, "--timestamp=none", "--generate-entitlement-der"}
	if entitlementsPath != "" {
		args = append(args, "--entitlements", entitlementsPath)
	}
	args = append(args, path)

	cmd := exec.CommandContext(context.Background(), "codesign", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign failed: %w\n%s", err, output)
	}
	return nil
}

// bundleIDFromProfile requires the exact team prefix; only a full wildcard
// preserves the original ID. Partial wildcards cannot be used as bundle IDs.
func bundleIDFromProfile(profile *provisioningProfile, originalBundleID string) (string, error) {
	if profile == nil || len(profile.TeamIdentifier) == 0 || profile.TeamIdentifier[0] == "" {
		return "", fmt.Errorf("no team identifier found in profile")
	}
	prefix := profile.TeamIdentifier[0] + "."
	appID := profile.Entitlements.ApplicationIdentifier
	if !strings.HasPrefix(appID, prefix) {
		return "", fmt.Errorf("profile application identifier %q must start with team prefix %q", appID, prefix)
	}
	bundleID := strings.TrimPrefix(appID, prefix)
	if bundleID == "*" {
		bundleID = originalBundleID
	}
	if bundleID == "" || strings.ContainsAny(bundleID, "* \t\r\n") {
		return "", fmt.Errorf("invalid bundle identifier %q in provisioning profile", bundleID)
	}
	return bundleID, nil
}

func configureBundleID(appPath string, profile *provisioningProfile, originalBundleID string, opts ResignOptions) error {
	if !opts.DeriveBundleIDFromProfile {
		return nil
	}
	bundleID, err := bundleIDFromProfile(profile, originalBundleID)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(bundleID, agentRunnerBundleID) {
		return fmt.Errorf("profile bundle ID %q must end with %q for agent discovery", bundleID, agentRunnerBundleID)
	}
	if bundleID == originalBundleID {
		return nil
	}
	if err := rewriteBundleID(appPath, bundleID); err != nil {
		return fmt.Errorf("failed to rewrite agent bundle ID: %w", err)
	}
	Verbose("Changed agent bundle ID from %s to %s", originalBundleID, bundleID)
	return nil
}

func rewriteBundleID(appPath, bundleID string) error {
	path := filepath.Join(appPath, "Info.plist")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read Info.plist: %w", err)
	}
	var info map[string]any
	format, err := plist.Unmarshal(data, &info)
	if err != nil || info == nil {
		return fmt.Errorf("failed to parse Info.plist dictionary: %v", err)
	}
	info["CFBundleIdentifier"] = bundleID
	data, err = plist.Marshal(info, format)
	if err != nil {
		return fmt.Errorf("failed to marshal Info.plist: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write Info.plist: %w", err)
	}
	return nil
}
