package devices

import "testing"

func TestAndroidContainerPathBuildsDataUserPath(t *testing.T) {
	cases := []struct {
		bundle, rel, want string
	}{
		{"com.example.app", "/files", "/data/user/0/com.example.app/files"},
		{"com.example.app", "files", "/data/user/0/com.example.app/files"},
		{"com.example.app", "", "/data/user/0/com.example.app"},
		{"com.example.app", "/databases/x.db", "/data/user/0/com.example.app/databases/x.db"},
	}
	for _, c := range cases {
		if got := androidContainerPath(c.bundle, c.rel); got != c.want {
			t.Errorf("androidContainerPath(%q,%q) = %q, want %q", c.bundle, c.rel, got, c.want)
		}
	}
}

// the container path must live under /data/user/, which is what makes
// buildShellCommand wrap the command in run-as and androidPackageName recover
// the package to run as.
func TestAndroidContainerPathIsRunAsWrapped(t *testing.T) {
	p := androidContainerPath("com.example.app", "/files/x")
	pkg, err := androidPackageName(p)
	if err != nil {
		t.Fatalf("androidPackageName(%q) error: %v", p, err)
	}
	if pkg != "com.example.app" {
		t.Errorf("recovered package = %q, want com.example.app", pkg)
	}
	cmd, err := (&AndroidDevice{}).buildShellCommand(p, "ls", p)
	if err != nil {
		t.Fatal(err)
	}
	if got := cmd[:7]; got != "run-as " {
		t.Errorf("command for a container path = %q…, want it to start with 'run-as '", got)
	}
}
