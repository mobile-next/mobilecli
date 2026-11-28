package devices

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// adbPathTestCase describes SDK dirs (relative to a fake $HOME, true = has
// adb), env vars pointing at them, and which one getAdbPath should pick
// ("" means fall back to "adb" from PATH).
type adbPathTestCase struct {
	name          string
	sdks          map[string]bool
	env           map[string]string
	want          string
	skipOnWindows bool
}

func adbIn(sdk string) string {
	name := "adb"
	if runtime.GOOS == goosWindows {
		name += ".exe"
	}
	return filepath.Join(sdk, "platform-tools", name)
}

func createFakeSdk(t *testing.T, dir string, withAdb bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "platform-tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	if withAdb {
		if err := os.WriteFile(adbIn(dir), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func useOnlyFakeHome(t *testing.T, home string) {
	t.Helper()
	for _, env := range []string{"ANDROID_HOME", "ANDROID_SDK_ROOT", "LOCALAPPDATA", "USERPROFILE"} {
		t.Setenv(env, "")
	}
	t.Setenv("HOME", home)
}

func TestGetAdbPath(t *testing.T) {
	cases := []adbPathTestCase{
		{name: "ANDROID_HOME", sdks: map[string]bool{"a": true}, env: map[string]string{"ANDROID_HOME": "a"}, want: "a"},
		{name: "ANDROID_SDK_ROOT", sdks: map[string]bool{"a": true}, env: map[string]string{"ANDROID_SDK_ROOT": "a"}, want: "a"},
		{name: "ANDROID_HOME wins over ANDROID_SDK_ROOT", sdks: map[string]bool{"a": true, "b": true}, env: map[string]string{"ANDROID_HOME": "a", "ANDROID_SDK_ROOT": "b"}, want: "a"},
		{name: "skips SDK without adb", sdks: map[string]bool{"a": false, "b": true}, env: map[string]string{"ANDROID_HOME": "a", "ANDROID_SDK_ROOT": "b"}, want: "b"},
		{name: "falls back to PATH", sdks: map[string]bool{"a": false}, env: map[string]string{"ANDROID_HOME": "a"}, want: ""},
		{name: "macOS default", sdks: map[string]bool{filepath.Join("Library", "Android", "sdk"): true}, want: filepath.Join("Library", "Android", "sdk")},
		{name: "Linux default", sdks: map[string]bool{filepath.Join("Android", "Sdk"): true}, want: filepath.Join("Android", "Sdk"), skipOnWindows: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipOnWindows && runtime.GOOS == goosWindows {
				t.Skip("not an SDK default on Windows")
			}
			home := t.TempDir()
			useOnlyFakeHome(t, home)
			for dir, withAdb := range tc.sdks {
				createFakeSdk(t, filepath.Join(home, dir), withAdb)
			}
			for env, dir := range tc.env {
				t.Setenv(env, filepath.Join(home, dir))
			}

			want := "adb"
			if tc.want != "" {
				want = adbIn(filepath.Join(home, tc.want))
			}
			if got := getAdbPath(); got != want {
				t.Fatalf("getAdbPath() = %q, want %q", got, want)
			}
		})
	}
}
