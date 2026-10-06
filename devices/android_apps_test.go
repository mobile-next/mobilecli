package devices

import (
	"errors"
	"testing"
)

func TestPmPathReportsAnInstalledPackage(t *testing.T) {
	installed, err := pmPathReportsInstalled([]byte("package:/system_ext/priv-app/SettingsGoogle/SettingsGoogle.apk\n"), nil)

	if err != nil || !installed {
		t.Fatalf("got installed=%v err=%v, want installed with no error", installed, err)
	}
}

func TestPmPathReportsAnInstalledSplitApkPackage(t *testing.T) {
	output := []byte("package:/data/app/com.example/base.apk\npackage:/data/app/com.example/split_config.arm64_v8a.apk\n")

	installed, err := pmPathReportsInstalled(output, nil)

	if err != nil || !installed {
		t.Fatalf("got installed=%v err=%v, want installed with no error", installed, err)
	}
}

func TestPmPathReportsAMissingPackageAsNotInstalled(t *testing.T) {
	// pm path exits 1 and prints nothing for a package that is not installed
	installed, err := pmPathReportsInstalled([]byte(""), errors.New("exit status 1"))

	if err != nil || installed {
		t.Fatalf("got installed=%v err=%v, want not installed with no error", installed, err)
	}
}

func TestPmPathReportsAnAdbFailureAsAnError(t *testing.T) {
	_, err := pmPathReportsInstalled([]byte("adb: device 'emulator-5554' not found"), errors.New("exit status 1"))

	if err == nil {
		t.Fatal("an adb failure must not be read as 'not installed'")
	}
}

func TestParsePackageListerOutput(t *testing.T) {
	output := []byte(`[{"packageName":"com.mobilenext.devicekit","appName":"DeviceKit","version":"1.2.5","versionCode":10205}]`)

	apps, err := parsePackageListerOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	want := InstalledAppInfo{PackageName: "com.mobilenext.devicekit", AppName: "DeviceKit", Version: "1.2.5", VersionCode: "10205"}
	if apps[0] != want {
		t.Errorf("got %+v, want %+v", apps[0], want)
	}
}

func TestParsePackageListerOutputRejectsNonJSON(t *testing.T) {
	if _, err := parsePackageListerOutput([]byte("Error: java.lang.NoSuchMethodException")); err == nil {
		t.Fatal("expected error for non-JSON output")
	}
}

func TestAppNameForReturnsLabel(t *testing.T) {
	apps := []InstalledAppInfo{
		{PackageName: "com.mobilenext.devicekit", AppName: "DeviceKit"},
		{PackageName: "com.mobilenext.playground", AppName: "Playground"},
	}

	if got := appNameFor(apps, "com.mobilenext.playground"); got != "Playground" {
		t.Errorf("got %q, want %q", got, "Playground")
	}
}

func TestAppNameForFallsBackToPackageNameWhenUnknown(t *testing.T) {
	apps := []InstalledAppInfo{{PackageName: "com.mobilenext.devicekit", AppName: "DeviceKit"}}

	if got := appNameFor(apps, "com.example.missing"); got != "com.example.missing" {
		t.Errorf("got %q, want the package name back", got)
	}
}

func TestAppNameForFallsBackWhenLabelIsEmpty(t *testing.T) {
	apps := []InstalledAppInfo{{PackageName: "com.example.nolabel", AppName: ""}}

	if got := appNameFor(apps, "com.example.nolabel"); got != "com.example.nolabel" {
		t.Errorf("got %q, want the package name back", got)
	}
}

func Test_adbShellArgs_keepsAPackageNameFromBecomingASecondCommand(t *testing.T) {
	args := adbShellArgs("am", "force-stop", "com.example; touch /sdcard/x")

	// adb joins everything after `shell` with spaces and lets the device's sh parse
	// it, so the whole command must arrive as one pre-quoted argument
	want := []string{"shell", `am force-stop 'com.example; touch /sdcard/x'`}
	if len(args) != len(want) || args[0] != want[0] || args[1] != want[1] {
		t.Errorf("got %q, want %q", args, want)
	}
}
