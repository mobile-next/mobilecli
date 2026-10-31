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
			if err := configureBundleID(app, profile, agentRunnerBundleID, ResignOptions{DeriveBundleIDFromProfile: true}); err != nil {
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
	if err := configureBundleID(t.TempDir(), profile, agentRunnerBundleID, ResignOptions{DeriveBundleIDFromProfile: true}); err == nil {
		t.Fatal("accepted unrelated profile")
	}
	for _, appID := range []string{"TEAM.*", "TEAM." + agentRunnerBundleID} {
		profile.Entitlements.ApplicationIdentifier = appID
		// No Info.plist exists: unchanged identifiers must not trigger a rewrite.
		if err := configureBundleID(t.TempDir(), profile, agentRunnerBundleID, ResignOptions{DeriveBundleIDFromProfile: true}); err != nil {
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
