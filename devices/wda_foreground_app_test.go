package devices

import (
	"testing"

	"github.com/mobile-next/mobilecli/devices/devicekit"
)

func activeApp(bundleID string, name string) *devicekit.ActiveAppInfo {
	return &devicekit.ActiveAppInfo{BundleID: bundleID, Name: name, ViewController: "SomeController"}
}

func TestForegroundAppNamesTheHomeScreenSpringBoardWhenWDAGivesABlankName(t *testing.T) {
	app := foregroundAppFromWDA(activeApp("com.apple.springboard", " "), nil)

	if app.AppName != "SpringBoard" {
		t.Errorf("got app name %q, want %q", app.AppName, "SpringBoard")
	}
}

func TestForegroundAppFallsBackToTheBundleIDWhenWDAGivesABlankName(t *testing.T) {
	app := foregroundAppFromWDA(activeApp("com.example.system", "  "), nil)

	if app.AppName != "com.example.system" {
		t.Errorf("got app name %q, want the bundle ID", app.AppName)
	}
}

func TestForegroundAppKeepsTheNameWDAReports(t *testing.T) {
	app := foregroundAppFromWDA(activeApp("com.example.system", "System Thing"), nil)

	if app.AppName != "System Thing" {
		t.Errorf("got app name %q, want %q", app.AppName, "System Thing")
	}
}

func TestForegroundAppPrefersTheInstalledAppEntry(t *testing.T) {
	installed := []InstalledAppInfo{{PackageName: "com.example.app", AppName: "Example", Version: "1.2"}}

	app := foregroundAppFromWDA(activeApp("com.example.app", " "), installed)

	if app.AppName != "Example" || app.Version != "1.2" || app.Activity != "SomeController" {
		t.Errorf("got %+v, want the installed app's name and version with WDA's activity", app)
	}
}
