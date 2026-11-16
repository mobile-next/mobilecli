package utils

import (
	"context"
	"crypto/sha1" // #nosec G505 -- Apple identifies signing identities by the certificate's SHA-1 fingerprint.
	"crypto/x509"
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
	// RequiredBundleIDSuffix, when set, renames the app to the profile's bundle ID,
	// which must end with this suffix. Empty keeps the IPA's original bundle ID.
	RequiredBundleIDSuffix string
}

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

	// a wildcard profile never renames the app, so bundleID is still the one being signed
	entitlementsPath, err := writeEntitlementsPlist(profile, bundleID)
	if err != nil {
		return "", fmt.Errorf("failed to extract entitlements: %w", err)
	}
	defer func() { _ = os.Remove(entitlementsPath) }()
	Verbose("Extracted entitlements to %s", entitlementsPath)

	embeddedPath := filepath.Join(appPath, "embedded.mobileprovision")
	if err = CopyFile(profilePath, embeddedPath); err != nil {
		return "", fmt.Errorf("failed to copy provisioning profile: %w", err)
	}

	if err = signAppBundle(appPath, identity, profile, entitlementsPath); err != nil {
		return "", err
	}

	return repackageIPA(tempDir)
}

// signAppBundle signs inside out: frameworks, then plugins, then the app itself.
func signAppBundle(appPath, identity string, profile *provisioningProfile, entitlementsPath string) error {
	if err := signFrameworks(appPath, identity); err != nil {
		return err
	}
	if err := signPlugins(appPath, identity, profile, entitlementsPath); err != nil {
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
func signPlugins(appPath, identity string, profile *provisioningProfile, appEntitlementsPath string) error {
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
		entitlementsPath, removeEntitlements, err := pluginEntitlements(profile, pluginPath, appEntitlementsPath)
		if err != nil {
			return err
		}
		Verbose("Signing plugin: %s", entry.Name())
		err = codesign(pluginPath, identity, entitlementsPath)
		removeEntitlements()
		if err != nil {
			return fmt.Errorf("failed to sign plugin %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// pluginEntitlements returns the entitlements file to sign a plugin with, and a func
// that removes it. Under a wildcard profile each plugin is named by its own bundle ID;
// an exact profile keeps signing plugins with the app's entitlements.
func pluginEntitlements(profile *provisioningProfile, pluginPath, appEntitlementsPath string) (string, func(), error) {
	if !hasWildcardApplicationIdentifier(profile) {
		return appEntitlementsPath, func() {}, nil
	}
	bundleID, err := readBundleID(pluginPath)
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to read bundle ID of plugin %s: %w", filepath.Base(pluginPath), err)
	}
	entitlementsPath, err := writeEntitlementsPlist(profile, bundleID)
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to write entitlements for plugin %s: %w", filepath.Base(pluginPath), err)
	}
	return entitlementsPath, func() { _ = os.Remove(entitlementsPath) }, nil
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
	if identityOverride == "" {
		return findSigningIdentity(profile)
	}
	identityOutput, err := listSigningIdentities()
	if err != nil {
		return "", err
	}
	if err := checkSigningIdentityOverride(identityOverride, identityOutput); err != nil {
		return "", err
	}
	return identityOverride, nil
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

// the values are ordered: chooseProfile ranks a higher match above a lower one.
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

// profileCandidate is a decoded provisioning profile and the file it was read from.
type profileCandidate struct {
	path    string
	profile *provisioningProfile
}

// usableProfileBonus ranks any profile the keychain can sign with above any it cannot.
const usableProfileBonus = int(profileMatchExact)

func findMatchingProfile(deviceUDID, bundleID string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	candidates := readProfiles([]string{
		filepath.Join(homeDir, "Library", "MobileDevice", "Provisioning Profiles"),
		filepath.Join(homeDir, "Library", "Developer", "Xcode", "UserData", "Provisioning Profiles"),
	})
	if len(candidates) == 0 {
		return "", noMatchingProfileError(deviceUDID, bundleID)
	}

	identityOutput, err := listSigningIdentities()
	if err != nil {
		return "", err
	}

	if path := chooseProfile(candidates, identityOutput, deviceUDID, bundleID); path != "" {
		return path, nil
	}
	return "", noMatchingProfileError(deviceUDID, bundleID)
}

// readProfiles decodes every provisioning profile in dirs; a missing dir has none.
func readProfiles(dirs []string) []profileCandidate {
	var candidates []profileCandidate
	for _, dir := range dirs {
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
			candidates = append(candidates, profileCandidate{path: fullPath, profile: profile})
		}
	}
	return candidates
}

// chooseProfile returns the path of the best profile for the device and bundle ID, or ""
// when none matches. A profile the keychain can sign with wins over one it cannot, then
// an exact bundle ID wins over a wildcard. Old profiles stay on disk after a certificate
// is revoked, so the first match is not always one that works. When no match is usable
// the best one is still returned, so that signing reports what is wrong with it.
func chooseProfile(candidates []profileCandidate, identityOutput, deviceUDID, bundleID string) string {
	best, bestScore := "", 0
	for _, candidate := range candidates {
		match := matchProfile(candidate.profile, deviceUDID, bundleID)
		if match == profileMatchNone {
			continue
		}
		score := int(match)
		if _, err := findProfileSigningIdentity(identityOutput, candidate.profile); err == nil {
			score += usableProfileBonus
		} else {
			Verbose("Profile %s has no usable signing certificate", candidate.profile.Name)
		}
		if score > bestScore {
			best, bestScore = candidate.path, score
		}
	}
	if best != "" {
		Verbose("Found matching profile: %s", best)
	}
	return best
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

func listSigningIdentities() (string, error) {
	cmd := exec.CommandContext(context.Background(), "/usr/bin/security", "find-identity", "-v", "-p", "codesigning")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list signing identities: %w", err)
	}
	return string(output), nil
}

func findSigningIdentity(profile *provisioningProfile) (string, error) {
	identityOutput, err := listSigningIdentities()
	if err != nil {
		return "", err
	}

	identity, err := findProfileSigningIdentity(identityOutput, profile)
	if err != nil {
		return "", err
	}
	Verbose("Found signing identity included in provisioning profile: %s", identity)
	return identity, nil
}

// findProfileSigningIdentity returns the first certificate of the profile that the
// keychain can sign with. When there is none, the error gives the reason for each one.
func findProfileSigningIdentity(identityOutput string, profile *provisioningProfile) (string, error) {
	keychainProblems := make(map[string]string)
	for _, identity := range parseSigningIdentities(identityOutput) {
		keychainProblems[identity.hash] = identity.problem
	}

	var unusable []string
	for _, certificate := range profile.DeveloperCertificates {
		hash := certificateHash(certificate)
		problem, inKeychain := keychainProblems[hash]
		if inKeychain && problem == "" {
			return hash, nil
		}
		unusable = append(unusable, describeUnusableCertificate(certificate, hash, problem))
	}

	return "", noUsableCertificateError(profile.Name, unusable)
}

// checkSigningIdentityOverride refuses a --signing-identity value that can only mean
// identities security reports a problem for: codesign would sign with it and the device
// would then reject the install. A value that means a usable identity, or none that
// security lists, is left to codesign.
func checkSigningIdentityOverride(identityOverride, identityOutput string) error {
	var unusable []string
	for _, identity := range identitiesNamedBy(identityOverride, parseSigningIdentities(identityOutput)) {
		if identity.problem == "" {
			return nil
		}
		unusable = append(unusable, fmt.Sprintf("%s (%s): %s", identity.name, identity.hash[:8], describeKeychainProblem(identity.problem)))
	}
	if len(unusable) == 0 {
		return nil
	}
	return fmt.Errorf(`signing identity %q cannot be used:
  - %s
to fix this, pass an identity that "security find-identity -v -p codesigning" lists without a status, or leave out --signing-identity`, identityOverride, strings.Join(unusable, "\n  - "))
}

// identitiesNamedBy returns the identities codesign would consider for a -s value: the
// ones with that hash or exactly that name, and only when there are none, the ones whose
// name contains it.
func identitiesNamedBy(value string, identities []signingIdentity) []signingIdentity {
	var exact, partial []signingIdentity
	for _, identity := range identities {
		switch {
		case strings.EqualFold(identity.hash, value) || identity.name == value:
			exact = append(exact, identity)
		case strings.Contains(identity.name, value):
			partial = append(partial, identity)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return partial
}

func certificateHash(certificate []byte) string {
	// #nosec G401 -- Apple identifies signing identities by the certificate's SHA-1 fingerprint.
	sum := sha1.Sum(certificate)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// signingIdentity is one identity listed by `security find-identity -v`. problem is the
// status security prints after the name and is empty for an identity that can sign.
type signingIdentity struct {
	hash, name, problem string
}

// parseSigningIdentities reads `security find-identity -v` output. security counts a
// revoked identity as valid and only marks it with a trailing status, as in
// `2) <hash> "<name>" (CSSMERR_TP_CERT_REVOKED)`.
func parseSigningIdentities(identityOutput string) []signingIdentity {
	var identities []signingIdentity
	for _, line := range strings.Split(identityOutput, "\n") {
		fields := strings.Fields(line)
		nameStart, nameEnd := strings.Index(line, `"`), strings.LastIndex(line, `"`)
		if len(fields) < 2 || len(fields[1]) != sha1.Size*2 || nameStart == nameEnd {
			continue
		}
		identities = append(identities, signingIdentity{
			hash:    strings.ToUpper(fields[1]),
			name:    line[nameStart+1 : nameEnd],
			problem: strings.Trim(strings.TrimSpace(line[nameEnd+1:]), "()"),
		})
	}
	return identities
}

func describeKeychainProblem(problem string) string {
	if problem == "CSSMERR_TP_CERT_REVOKED" {
		return "revoked"
	}
	return problem
}

// describeUnusableCertificate says why a profile certificate cannot sign. keychainProblem
// is the status security gave it, empty when security did not list it. security leaves
// out an expired identity, so expiry is read from the certificate itself.
func describeUnusableCertificate(certificate []byte, hash, keychainProblem string) string {
	name := "certificate"
	reason := "not in the keychain, or its private key is missing"

	if parsed, err := x509.ParseCertificate(certificate); err == nil {
		name = parsed.Subject.CommonName
		if parsed.NotAfter.Before(time.Now()) {
			reason = "expired on " + parsed.NotAfter.UTC().Format(time.DateOnly)
		}
	}
	if keychainProblem != "" {
		reason = describeKeychainProblem(keychainProblem)
	}

	return fmt.Sprintf("%s (%s): %s", name, hash[:8], reason)
}

func noUsableCertificateError(profileName string, unusable []string) error {
	certificates := "  the profile lists no certificates"
	if len(unusable) > 0 {
		certificates = "  - " + strings.Join(unusable, "\n  - ")
	}
	return fmt.Errorf(`provisioning profile %q has no signing certificate that can be used:
%s
to fix this, either:
a) build this app for your device in Xcode (refreshes the profile and its certificate)
b) download a profile that includes a valid certificate from developer.apple.com/account and pass it with --provisioning-profile
c) if a certificate is valid but not in the keychain, import it together with its private key`, profileName, certificates)
}

func hasWildcardApplicationIdentifier(profile *provisioningProfile) bool {
	return strings.HasSuffix(profile.Entitlements.ApplicationIdentifier, ".*")
}

// applicationIdentifierFor resolves a wildcard profile identifier to the bundle being
// signed. iOS installs the literal wildcard once, then rejects it as an upgrade over an
// app that has its real identifier. The prefix comes from the profile because an older
// account's App ID prefix is not always its team ID.
func applicationIdentifierFor(profile *provisioningProfile, bundleID string) string {
	appID := profile.Entitlements.ApplicationIdentifier
	if !hasWildcardApplicationIdentifier(profile) {
		return appID
	}
	prefix, _, _ := strings.Cut(appID, ".")
	return prefix + "." + bundleID
}

func writeEntitlementsPlist(profile *provisioningProfile, bundleID string) (string, error) {
	entitlementsMap := map[string]any{
		"application-identifier": applicationIdentifierFor(profile, bundleID),
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

// configureBundleID optionally derives the bundle ID from the profile, validates
// the required suffix, and rewrites the main bundle only when the ID changes.
func configureBundleID(appPath string, profile *provisioningProfile, originalBundleID string, opts ResignOptions) error {
	if opts.RequiredBundleIDSuffix == "" {
		return nil
	}
	bundleID, err := bundleIDFromProfile(profile, originalBundleID)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(bundleID, opts.RequiredBundleIDSuffix) {
		return fmt.Errorf("profile bundle ID %q must end with %q", bundleID, opts.RequiredBundleIDSuffix)
	}
	if bundleID == originalBundleID {
		return nil
	}
	if err := rewriteBundleID(appPath, bundleID); err != nil {
		return fmt.Errorf("failed to rewrite bundle ID: %w", err)
	}
	Verbose("Changed bundle ID from %s to %s", originalBundleID, bundleID)
	return nil
}

// rewriteBundleID replaces CFBundleIdentifier in the main app Info.plist while
// preserving its format and other properties. It returns read, parse, or write errors.
func rewriteBundleID(appPath, bundleID string) error {
	path := filepath.Join(appPath, "Info.plist")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read Info.plist: %w", err)
	}
	var info map[string]any
	format, err := plist.Unmarshal(data, &info)
	if err != nil {
		return fmt.Errorf("failed to parse Info.plist: %w", err)
	}
	if info == nil {
		return fmt.Errorf("Info.plist is not a dictionary")
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
