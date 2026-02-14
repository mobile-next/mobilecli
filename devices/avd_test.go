package devices

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConvertAPILevelToVersion(t *testing.T) {
	tests := []struct {
		apiLevel string
		want     string
	}{
		{"36", "16.0"},
		{"35", "15.0"},
		{"34", "14.0"},
		{"33", "13.0"},
		{"32", "12.1"},
		{"31", "12.0"},
		{"30", "11.0"},
		{"29", "10.0"},
		{"28", "9.0"},
		{"21", "5.0"},
		// unknown API level returns as-is
		{"99", "99"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run("api_"+tt.apiLevel, func(t *testing.T) {
			if got := convertAPILevelToVersion(tt.apiLevel); got != tt.want {
				t.Errorf("convertAPILevelToVersion(%q) = %q, want %q", tt.apiLevel, got, tt.want)
			}
		})
	}
}

func TestGetAVDDetails_WithFixtures(t *testing.T) {
	// create a temporary .android/avd directory structure
	tmpHome := t.TempDir()

	avdDir := filepath.Join(tmpHome, ".android", "avd")
	if err := os.MkdirAll(avdDir, 0750); err != nil {
		t.Fatal(err)
	}

	// create a .avd directory with config.ini
	avdDataDir := filepath.Join(avdDir, "Pixel_9_Pro.avd")
	if err := os.MkdirAll(avdDataDir, 0750); err != nil {
		t.Fatal(err)
	}

	// write the top-level .ini file pointing to the .avd directory
	iniContent := "path=" + avdDataDir + "\n"
	if err := os.WriteFile(filepath.Join(avdDir, "Pixel_9_Pro.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatal(err)
	}

	// write config.ini inside the .avd directory
	configContent := `avd.ini.displayname=Pixel 9 Pro
target=android-36
AvdId=Pixel_9_Pro
`
	if err := os.WriteFile(filepath.Join(avdDataDir, "config.ini"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// override HOME for this test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	details, err := getAVDDetails()
	if err != nil {
		t.Fatalf("getAVDDetails() error: %v", err)
	}

	if len(details) != 1 {
		t.Fatalf("expected 1 AVD, got %d", len(details))
	}

	avd, ok := details["Pixel_9_Pro"]
	if !ok {
		t.Fatal("expected AVD 'Pixel_9_Pro' in results")
	}

	if avd.Name != "Pixel 9 Pro" {
		t.Errorf("Name = %q, want %q", avd.Name, "Pixel 9 Pro")
	}
	if avd.APILevel != "36" {
		t.Errorf("APILevel = %q, want %q", avd.APILevel, "36")
	}
	if avd.AvdId != "Pixel_9_Pro" {
		t.Errorf("AvdId = %q, want %q", avd.AvdId, "Pixel_9_Pro")
	}
}

func TestGetAVDDetails_EmptyDirectory(t *testing.T) {
	tmpHome := t.TempDir()

	avdDir := filepath.Join(tmpHome, ".android", "avd")
	if err := os.MkdirAll(avdDir, 0750); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	details, err := getAVDDetails()
	if err != nil {
		t.Fatalf("getAVDDetails() error: %v", err)
	}

	if len(details) != 0 {
		t.Errorf("expected 0 AVDs, got %d", len(details))
	}
}

func TestGetAVDDetails_SkipsMissingDisplayName(t *testing.T) {
	tmpHome := t.TempDir()

	avdDir := filepath.Join(tmpHome, ".android", "avd")
	avdDataDir := filepath.Join(avdDir, "broken.avd")
	if err := os.MkdirAll(avdDataDir, 0750); err != nil {
		t.Fatal(err)
	}

	iniContent := "path=" + avdDataDir + "\n"
	if err := os.WriteFile(filepath.Join(avdDir, "broken.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatal(err)
	}

	// config.ini without avd.ini.displayname
	configContent := "target=android-31\n"
	if err := os.WriteFile(filepath.Join(avdDataDir, "config.ini"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	details, err := getAVDDetails()
	if err != nil {
		t.Fatalf("getAVDDetails() error: %v", err)
	}

	if len(details) != 0 {
		t.Errorf("expected 0 AVDs (missing display name), got %d", len(details))
	}
}

func TestGetOfflineAndroidEmulators(t *testing.T) {
	tmpHome := t.TempDir()

	avdDir := filepath.Join(tmpHome, ".android", "avd")
	avdDataDir := filepath.Join(avdDir, "TestEmu.avd")
	if err := os.MkdirAll(avdDataDir, 0750); err != nil {
		t.Fatal(err)
	}

	iniContent := "path=" + avdDataDir + "\n"
	if err := os.WriteFile(filepath.Join(avdDir, "TestEmu.ini"), []byte(iniContent), 0644); err != nil {
		t.Fatal(err)
	}

	configContent := `avd.ini.displayname=Test Emulator
target=android-36
AvdId=TestEmu
`
	if err := os.WriteFile(filepath.Join(avdDataDir, "config.ini"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// no online devices - emulator should show as offline
	offlineDevices, err := getOfflineAndroidEmulators(map[string]bool{})
	if err != nil {
		t.Fatalf("getOfflineAndroidEmulators() error: %v", err)
	}

	if len(offlineDevices) != 1 {
		t.Fatalf("expected 1 offline device, got %d", len(offlineDevices))
	}

	device := offlineDevices[0]
	if device.ID() != "TestEmu" {
		t.Errorf("ID() = %q, want %q", device.ID(), "TestEmu")
	}
	if device.State() != "offline" {
		t.Errorf("State() = %q, want %q", device.State(), "offline")
	}
	if device.Version() != "16.0" {
		t.Errorf("Version() = %q, want %q", device.Version(), "16.0")
	}

	// mark as online - should not appear in offline list
	offlineDevices, err = getOfflineAndroidEmulators(map[string]bool{"TestEmu": true})
	if err != nil {
		t.Fatalf("getOfflineAndroidEmulators() error: %v", err)
	}

	if len(offlineDevices) != 0 {
		t.Errorf("expected 0 offline devices when online, got %d", len(offlineDevices))
	}
}
