package cli

import "testing"

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
