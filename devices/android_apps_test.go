package devices

import "testing"

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
