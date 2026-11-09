package utils

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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
