package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"howett.net/plist"
)

// an Xcode debug build keeps the app's code in <App>.debug.dylib next to a stub
// executable, plus __preview.dylib for SwiftUI previews.
func xcodeDebugAppBundle(t *testing.T) string {
	t.Helper()
	appPath := filepath.Join(t.TempDir(), "Runner.app")
	files := []string{
		"Runner",
		"Info.plist",
		"Runner.debug.dylib",
		"__preview.dylib",
		"Frameworks/libswiftCore.dylib",
		"Frameworks/Flutter.framework/Flutter",
		"PlugIns/Widget.appex/Widget.debug.dylib",
	}
	for _, name := range files {
		path := filepath.Join(appPath, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return appPath
}

func TestLooseDylibsFindsTheDylibsXcodeDebugBuildsPutInTheAppRoot(t *testing.T) {
	appPath := xcodeDebugAppBundle(t)

	found, err := looseDylibs(appPath)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{filepath.Join(appPath, "Runner.debug.dylib"), filepath.Join(appPath, "__preview.dylib")}
	if !slices.Equal(found, want) {
		t.Fatalf("expected only the app root's dylibs\n got %v\nwant %v", found, want)
	}
}

func TestLooseDylibsAlsoWorksOnAnAppExtension(t *testing.T) {
	extensionPath := filepath.Join(xcodeDebugAppBundle(t), "PlugIns", "Widget.appex")

	found, err := looseDylibs(extensionPath)

	if err != nil || !slices.Equal(found, []string{filepath.Join(extensionPath, "Widget.debug.dylib")}) {
		t.Fatalf("expected the extension's own debug dylib, got %v, %v", found, err)
	}
}

func TestLooseDylibsIsEmptyForAReleaseBuild(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "Runner.app")
	if err := os.MkdirAll(appPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appPath, "Runner"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := looseDylibs(appPath)

	if err != nil || len(found) != 0 {
		t.Fatalf("expected no dylibs and no error, got %v, %v", found, err)
	}
}

const agentRunnerBundleID = "com.mobilenext.devicekit-iosUITests.xctrunner"

// TestBundleIDFromProfile covers explicit and wildcard IDs and invalid profile identifiers.
func TestBundleIDFromProfile(t *testing.T) {
	for _, tc := range []struct {
		name        string
		teams       []string
		appID, want string
		wantErr     bool
	}{
		{"original", []string{"TEAM"}, "TEAM." + agentRunnerBundleID, agentRunnerBundleID, false},
		{"custom", []string{"TEAM"}, "TEAM.custom." + agentRunnerBundleID, "custom." + agentRunnerBundleID, false},
		{"wildcard", []string{"TEAM"}, "TEAM.*", agentRunnerBundleID, false},
		{"missing team", nil, "TEAM.test", "", true},
		{"empty team", []string{""}, ".test", "", true},
		{"wrong prefix", []string{"TEAM"}, "OTHER.test", "", true},
		{"missing app ID", []string{"TEAM"}, "", "", true},
		{"empty bundle ID", []string{"TEAM"}, "TEAM.", "", true},
		{"partial wildcard", []string{"TEAM"}, "TEAM.custom.*", "", true},
		{"whitespace", []string{"TEAM"}, "TEAM.bad id", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := &provisioningProfile{TeamIdentifier: tc.teams, Entitlements: entitlements{ApplicationIdentifier: tc.appID}}
			got, err := bundleIDFromProfile(profile, agentRunnerBundleID)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got %q, %v; want %q, error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// TestRewriteBundleID verifies XML and binary plist preservation and opt-in rewriting.
func TestRewriteBundleID(t *testing.T) {
	for _, format := range []int{plist.XMLFormat, plist.BinaryFormat} {
		t.Run(fmt.Sprint(format), func(t *testing.T) {
			app := t.TempDir()
			path := filepath.Join(app, "Info.plist")
			info := map[string]any{"CFBundleIdentifier": agentRunnerBundleID, "CFBundleName": "Runner", "Settings": map[string]any{"enabled": true}, "Values": []any{"one", "two"}}
			data, err := plist.Marshal(info, format)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			profile := &provisioningProfile{TeamIdentifier: []string{"TEAM"}, Entitlements: entitlements{ApplicationIdentifier: "TEAM.custom." + agentRunnerBundleID}}
			if err := configureBundleID(app, profile, agentRunnerBundleID, ResignOptions{}); err != nil {
				t.Fatal(err)
			}
			unchanged, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, unchanged) {
				t.Fatalf("disabled option changed plist: %v", err)
			}
			if err := configureBundleID(app, profile, agentRunnerBundleID, ResignOptions{RequiredBundleIDSuffix: agentRunnerBundleID}); err != nil {
				t.Fatal(err)
			}
			updated, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			gotFormat, err := plist.Unmarshal(updated, &got)
			info["CFBundleIdentifier"] = "custom." + agentRunnerBundleID
			if err != nil || gotFormat != format || !reflect.DeepEqual(got, info) {
				t.Fatalf("unexpected plist: %#v, format %d, error %v", got, gotFormat, err)
			}
		})
	}
}

// TestConfigureBundleIDValidation checks suffix validation and skips rewriting unchanged IDs.
func TestConfigureBundleIDValidation(t *testing.T) {
	profile := &provisioningProfile{TeamIdentifier: []string{"TEAM"}, Entitlements: entitlements{ApplicationIdentifier: "TEAM.unrelated"}}
	if err := configureBundleID(t.TempDir(), profile, agentRunnerBundleID, ResignOptions{RequiredBundleIDSuffix: agentRunnerBundleID}); err == nil {
		t.Fatal("accepted unrelated profile")
	}
	for _, appID := range []string{"TEAM.*", "TEAM." + agentRunnerBundleID} {
		profile.Entitlements.ApplicationIdentifier = appID
		// No Info.plist exists: unchanged identifiers must not trigger a rewrite.
		if err := configureBundleID(t.TempDir(), profile, agentRunnerBundleID, ResignOptions{RequiredBundleIDSuffix: agentRunnerBundleID}); err != nil {
			t.Fatal(err)
		}
	}
}

// TestRewriteBundleIDErrors checks missing and malformed plist failures.
func TestRewriteBundleIDErrors(t *testing.T) {
	app := t.TempDir()
	if err := rewriteBundleID(app, "custom"); err == nil {
		t.Fatal("accepted missing plist")
	}
	if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte("invalid plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rewriteBundleID(app, "custom"); err == nil {
		t.Fatal("accepted malformed plist")
	}
}

const agentTestPluginBundleID = "com.mobilenext.devicekit-iosUITests"

func profileWithApplicationIdentifier(appID string) *provisioningProfile {
	return &provisioningProfile{TeamIdentifier: []string{"TEAM"}, Entitlements: entitlements{ApplicationIdentifier: appID}}
}

func applicationIdentifierWrittenTo(t *testing.T, entitlementsPath string) string {
	t.Helper()
	data, err := os.ReadFile(entitlementsPath)
	if err != nil {
		t.Fatal(err)
	}
	var written map[string]any
	if _, err := plist.Unmarshal(data, &written); err != nil {
		t.Fatal(err)
	}
	appID, _ := written["application-identifier"].(string)
	return appID
}

func pluginWithBundleID(t *testing.T, bundleID string) string {
	t.Helper()
	pluginPath := filepath.Join(t.TempDir(), "Runner.xctest")
	if err := os.MkdirAll(pluginPath, 0o750); err != nil {
		t.Fatal(err)
	}
	data, err := plist.Marshal(map[string]any{"CFBundleIdentifier": bundleID}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginPath, "Info.plist"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return pluginPath
}

// iOS rejects an upgrade signed with the literal wildcard identifier, so a wildcard
// profile must be resolved to the bundle being signed. Exact profiles stay as they are.
func TestEntitlementsNameTheBundleBeingSigned(t *testing.T) {
	for _, tc := range []struct {
		name, profileAppID, want string
	}{
		{"wildcard profile", "TEAM.*", "TEAM." + agentRunnerBundleID},
		{"partial wildcard profile", "TEAM.com.mobilenext.*", "TEAM." + agentRunnerBundleID},
		{"app ID prefix that is not the team ID", "PREFIX.*", "PREFIX." + agentRunnerBundleID},
		{"exact profile", "TEAM.com.example.other", "TEAM.com.example.other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entitlementsPath, err := writeEntitlementsPlist(profileWithApplicationIdentifier(tc.profileAppID), agentRunnerBundleID)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(entitlementsPath) }()

			if got := applicationIdentifierWrittenTo(t, entitlementsPath); got != tc.want {
				t.Fatalf("application-identifier = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWildcardProfileGivesAPluginItsOwnIdentifier(t *testing.T) {
	pluginPath := pluginWithBundleID(t, agentTestPluginBundleID)

	entitlementsPath, removeEntitlements, err := pluginEntitlements(profileWithApplicationIdentifier("TEAM.*"), pluginPath, "app-entitlements.plist")
	if err != nil {
		t.Fatal(err)
	}
	defer removeEntitlements()

	if got, want := applicationIdentifierWrittenTo(t, entitlementsPath), "TEAM."+agentTestPluginBundleID; got != want {
		t.Fatalf("application-identifier = %q, want %q", got, want)
	}
}

func TestExactProfileSignsAPluginWithTheAppEntitlements(t *testing.T) {
	pluginPath := pluginWithBundleID(t, agentTestPluginBundleID)

	entitlementsPath, removeEntitlements, err := pluginEntitlements(profileWithApplicationIdentifier("TEAM."+agentRunnerBundleID), pluginPath, "app-entitlements.plist")
	if err != nil {
		t.Fatal(err)
	}
	defer removeEntitlements()

	if entitlementsPath != "app-entitlements.plist" {
		t.Fatalf("entitlements path = %q, want the app's", entitlementsPath)
	}
}

func TestWildcardProfileRejectsAPluginWithoutABundleID(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "Broken.appex")
	if err := os.MkdirAll(pluginPath, 0o750); err != nil {
		t.Fatal(err)
	}

	if _, _, err := pluginEntitlements(profileWithApplicationIdentifier("TEAM.*"), pluginPath, "app-entitlements.plist"); err == nil {
		t.Fatal("expected an error for a plugin with no Info.plist")
	}
}
