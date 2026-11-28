package devices

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeSdk creates dir/platform-tools/adb and returns dir.
func fakeSdk(t *testing.T, dir string, withAdb bool) string {
	t.Helper()
	tools := filepath.Join(dir, "platform-tools")
	if err := os.MkdirAll(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	if withAdb {
		name := "adb"
		if runtime.GOOS == goosWindows {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(tools, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func clearSdkEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("ANDROID_HOME", "")
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("USERPROFILE", "")
}

func adbIn(sdk string) string {
	name := "adb"
	if runtime.GOOS == goosWindows {
		name += ".exe"
	}
	return filepath.Join(sdk, "platform-tools", name)
}

func TestGetAdbPath_AndroidHome(t *testing.T) {
	clearSdkEnv(t, t.TempDir())
	sdk := fakeSdk(t, t.TempDir(), true)
	t.Setenv("ANDROID_HOME", sdk)

	if got := getAdbPath(); got != adbIn(sdk) {
		t.Fatalf("getAdbPath() = %q, want %q", got, adbIn(sdk))
	}
}

func TestGetAdbPath_AndroidSdkRoot(t *testing.T) {
	clearSdkEnv(t, t.TempDir())
	sdk := fakeSdk(t, t.TempDir(), true)
	t.Setenv("ANDROID_SDK_ROOT", sdk)

	if got := getAdbPath(); got != adbIn(sdk) {
		t.Fatalf("getAdbPath() = %q, want %q", got, adbIn(sdk))
	}
	if got := getAndroidSdkPath(); got != sdk {
		t.Fatalf("getAndroidSdkPath() = %q, want %q", got, sdk)
	}
}

func TestGetAdbPath_AndroidHomeWins(t *testing.T) {
	clearSdkEnv(t, t.TempDir())
	home := fakeSdk(t, t.TempDir(), true)
	root := fakeSdk(t, t.TempDir(), true)
	t.Setenv("ANDROID_HOME", home)
	t.Setenv("ANDROID_SDK_ROOT", root)

	if got := getAdbPath(); got != adbIn(home) {
		t.Fatalf("getAdbPath() = %q, want ANDROID_HOME's %q", got, adbIn(home))
	}
}

func TestGetAdbPath_SkipsSdkWithoutAdb(t *testing.T) {
	clearSdkEnv(t, t.TempDir())
	noAdb := fakeSdk(t, t.TempDir(), false)
	withAdb := fakeSdk(t, t.TempDir(), true)
	t.Setenv("ANDROID_HOME", noAdb)
	t.Setenv("ANDROID_SDK_ROOT", withAdb)

	if got := getAdbPath(); got != adbIn(withAdb) {
		t.Fatalf("getAdbPath() = %q, want %q", got, adbIn(withAdb))
	}
}

func TestGetAdbPath_FallsBackToPath(t *testing.T) {
	home := t.TempDir()
	clearSdkEnv(t, home)
	t.Setenv("ANDROID_HOME", fakeSdk(t, t.TempDir(), false))

	if got := getAdbPath(); got != "adb" {
		t.Fatalf("getAdbPath() = %q, want %q", got, "adb")
	}
}

func TestGetAdbPath_LinuxDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Android Studio's ~/Android/Sdk default is Linux-only")
	}
	home := t.TempDir()
	clearSdkEnv(t, home)
	sdk := fakeSdk(t, filepath.Join(home, "Android", "Sdk"), true)

	if got := getAdbPath(); got != adbIn(sdk) {
		t.Fatalf("getAdbPath() = %q, want %q", got, adbIn(sdk))
	}
	if got := getAndroidSdkPath(); got != sdk {
		t.Fatalf("getAndroidSdkPath() = %q, want %q", got, sdk)
	}
}

func TestGetAdbPath_MacDefault(t *testing.T) {
	home := t.TempDir()
	clearSdkEnv(t, home)
	sdk := fakeSdk(t, filepath.Join(home, "Library", "Android", "sdk"), true)

	if got := getAdbPath(); got != adbIn(sdk) {
		t.Fatalf("getAdbPath() = %q, want %q", got, adbIn(sdk))
	}
}
