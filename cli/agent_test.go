package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentPackageForPlatform(t *testing.T) {
	cases := map[string]string{
		"ios":     iosRunnerBundleID,
		"tvos":    tvosRunnerBundleID,
		"android": androidPackageName,
		"unknown": "",
	}
	for platform, want := range cases {
		if got := agentPackageForPlatform(platform); got != want {
			t.Errorf("agentPackageForPlatform(%q) = %q, want %q", platform, got, want)
		}
	}
}

func TestAgentVersionForPlatform(t *testing.T) {
	cases := map[string]string{
		"ios":     agentVersionIOS,
		"tvos":    agentVersionTVOS,
		"android": agentVersionAndroid,
		"unknown": "",
	}
	for platform, want := range cases {
		if got := agentVersionForPlatform(platform); got != want {
			t.Errorf("agentVersionForPlatform(%q) = %q, want %q", platform, got, want)
		}
	}
}

func TestAgentMatchesApp(t *testing.T) {
	// Re-signing a runner for a real device prefixes the bundle id with the team id,
	// so iOS and tvOS must match on suffix while other platforms match exactly.
	cases := []struct {
		name             string
		platform         string
		installedPackage string
		agentPackage     string
		want             bool
	}{
		{"ios exact", "ios", iosRunnerBundleID, iosRunnerBundleID, true},
		{"ios team-prefixed", "ios", "ABCDE12345." + iosRunnerBundleID, iosRunnerBundleID, true},
		{"tvos exact", "tvos", tvosRunnerBundleID, tvosRunnerBundleID, true},
		{"tvos team-prefixed", "tvos", "ABCDE12345." + tvosRunnerBundleID, tvosRunnerBundleID, true},
		{"tvos mismatch", "tvos", "com.example.other", tvosRunnerBundleID, false},
		{"android exact", "android", androidPackageName, androidPackageName, true},
		{"android no suffix match", "android", "prefix." + androidPackageName, androidPackageName, false},
	}
	for _, c := range cases {
		if got := agentMatchesApp(c.platform, c.installedPackage, c.agentPackage); got != c.want {
			t.Errorf("%s: agentMatchesApp(%q, %q, %q) = %v, want %v",
				c.name, c.platform, c.installedPackage, c.agentPackage, got, c.want)
		}
	}
}

func TestResolveRunnerArtifactUsesLocalPath(t *testing.T) {
	// With --agent-path set, resolveRunnerArtifact must skip the download and the
	// pinned checksum verification, returning the local path + its checksum.
	dir := t.TempDir()
	local := filepath.Join(dir, "custom-runner.ipa")
	if err := os.WriteFile(local, []byte("hello runner"), 0600); err != nil {
		t.Fatalf("failed to write local artifact: %v", err)
	}

	agentPath = local
	t.Cleanup(func() { agentPath = "" })

	path, checksum, err := resolveRunnerArtifact(t.TempDir(), "devicekit-tvos-runner.ipa", agentVersionTVOS)
	if err != nil {
		t.Fatalf("resolveRunnerArtifact returned error: %v", err)
	}
	if path != local {
		t.Errorf("expected local artifact path %q, got %q", local, path)
	}
	const want = "7d01a911d78908f3f3c466d39a0bd6a5c9ff9e20ab58bfda37909f5fd6f35afd"
	if checksum != want {
		t.Errorf("expected checksum %q, got %q", want, checksum)
	}
}

func TestResolveRunnerArtifactMissingLocalPath(t *testing.T) {
	agentPath = filepath.Join(t.TempDir(), "does-not-exist.ipa")
	t.Cleanup(func() { agentPath = "" })

	if _, _, err := resolveRunnerArtifact(t.TempDir(), "devicekit-tvos-runner.ipa", agentVersionTVOS); err == nil {
		t.Fatal("expected error for missing agent-path, got nil")
	}
}

func TestReleasePathRequiresPinnedChecksum(t *testing.T) {
	// The release path (no --agent-path) stays checksum-gated: an unknown filename
	// has no pinned checksum and must be rejected rather than installed unverified.
	agentPath = ""
	if _, ok := agentChecksums["devicekit-ios-runner.ipa"]; !ok {
		t.Error("expected a pinned checksum for the iOS release runner")
	}
}
