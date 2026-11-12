package devices

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const windowTestUDID = "13ED70EB-D993-4E69-B6AD-9764AD4D5E3A"

// xcodeWithApps creates a fake Xcode.app holding the given apps (paths relative
// to Xcode.app/Contents) and returns its developer directory
func xcodeWithApps(t *testing.T, apps ...string) string {
	t.Helper()

	contents := filepath.Join(t.TempDir(), "Xcode.app", "Contents")
	developerDir := filepath.Join(contents, "Developer")
	for _, dir := range append(apps, "Developer") {
		if err := os.MkdirAll(filepath.Join(contents, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return developerDir
}

func TestWindowOpensSimulatorAppOnTheBootedDevice(t *testing.T) {
	developerDir := xcodeWithApps(t, "Developer/Applications/Simulator.app")

	args, err := simulatorWindowCommand(developerDir, windowTestUDID)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{filepath.Join(developerDir, "Applications", "Simulator.app"), "--args", "-CurrentDeviceUDID", windowTestUDID}
	if !slices.Equal(args, expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
}

func TestWindowOpensDeviceHubOnTheBootedDeviceWhenXcodeHasNoSimulatorApp(t *testing.T) {
	developerDir := xcodeWithApps(t, "Applications/DeviceHub.app")

	args, err := simulatorWindowCommand(developerDir, windowTestUDID)
	if err != nil {
		t.Fatal(err)
	}

	deviceHubApp := filepath.Join(filepath.Dir(developerDir), "Applications", "DeviceHub.app")
	expected := []string{"-a", deviceHubApp, "devices://device/open?id=" + windowTestUDID}
	if !slices.Equal(args, expected) {
		t.Fatalf("expected %v, got %v", expected, args)
	}
}

func TestWindowFailsWhenXcodeHasNoSimulatorWindowApp(t *testing.T) {
	developerDir := xcodeWithApps(t)

	if _, err := simulatorWindowCommand(developerDir, windowTestUDID); err == nil {
		t.Fatal("expected an error when neither Simulator.app nor DeviceHub.app exists")
	}
}
