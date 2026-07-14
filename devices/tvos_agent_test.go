package devices

import (
	"os"
	"path/filepath"
	"testing"

	"howett.net/plist"
)

func TestTVOSRunnerCacheKeyVaries(t *testing.T) {
	base := tvosRunnerCacheKey("checksum-a", "udid-1", "profile-1")
	if base == "" {
		t.Fatal("expected a non-empty cache key")
	}
	// Same inputs are stable.
	if base != tvosRunnerCacheKey("checksum-a", "udid-1", "profile-1") {
		t.Error("expected cache key to be deterministic")
	}
	// Any input change alters the key.
	for _, other := range []string{
		tvosRunnerCacheKey("checksum-b", "udid-1", "profile-1"),
		tvosRunnerCacheKey("checksum-a", "udid-2", "profile-1"),
		tvosRunnerCacheKey("checksum-a", "udid-1", "profile-2"),
	} {
		if other == base {
			t.Error("expected cache key to change when an input changes")
		}
	}
}

func TestWriteTVOSXctestrun(t *testing.T) {
	cacheDir := t.TempDir()
	runnerApp := filepath.Join(cacheDir, "Payload", "devicekit-tvos-Runner.app")
	xctest := filepath.Join(runnerApp, "PlugIns", "devicekit-tvosUITests.xctest")
	xctestrunPath := filepath.Join(cacheDir, tvosRunnerXctestrunName)

	if err := writeTVOSXctestrun(xctestrunPath, cacheDir, runnerApp, xctest); err != nil {
		t.Fatalf("writeTVOSXctestrun returned error: %v", err)
	}

	data, err := os.ReadFile(xctestrunPath)
	if err != nil {
		t.Fatalf("failed to read xctestrun: %v", err)
	}

	var doc map[string]map[string]any
	if _, err := plist.Unmarshal(data, &doc); err != nil {
		t.Fatalf("failed to parse generated xctestrun: %v", err)
	}

	target, ok := doc["devicekit-tvosUITests"]
	if !ok {
		t.Fatalf("expected a devicekit-tvosUITests target, got keys %v", keysOf(doc))
	}
	if got := target["TestHostPath"]; got != "__TESTROOT__/Payload/devicekit-tvos-Runner.app" {
		t.Errorf("unexpected TestHostPath: %v", got)
	}
	if got := target["TestBundlePath"]; got != "__TESTHOST__/PlugIns/devicekit-tvosUITests.xctest" {
		t.Errorf("unexpected TestBundlePath: %v", got)
	}
	if got, _ := target["IsUITestBundle"].(bool); !got {
		t.Errorf("expected IsUITestBundle true, got %v", target["IsUITestBundle"])
	}
}

func keysOf(m map[string]map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestTVOSCacheKeyMatchesDevice(t *testing.T) {
	dir := t.TempDir()
	udid := "udid-1"
	keyPath := filepath.Join(dir, tvosRunnerCacheKeyFile)

	// A well-formed key file whose device token matches is accepted.
	contents := tvosRunnerDeviceKey(udid) + "\n" + tvosRunnerCacheKey("checksum-a", udid, "profile-1") + "\n"
	if err := os.WriteFile(keyPath, []byte(contents), 0600); err != nil {
		t.Fatalf("failed to write cache key: %v", err)
	}
	if !tvosCacheKeyMatchesDevice(dir, udid) {
		t.Error("expected matching device token to be accepted")
	}

	// A cache produced for a different device must be rejected.
	if tvosCacheKeyMatchesDevice(dir, "udid-2") {
		t.Error("expected device mismatch to be rejected")
	}

	// A missing cache key file must be rejected.
	if tvosCacheKeyMatchesDevice(t.TempDir(), udid) {
		t.Error("expected missing cache key to be rejected")
	}
}
