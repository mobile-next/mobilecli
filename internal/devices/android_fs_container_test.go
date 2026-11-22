package devices

import (
	"strings"
	"testing"
)

func TestAndroidContainerPathBuildsDataUserPath(t *testing.T) {
	cases := []struct {
		bundle, rel, want string
	}{
		{"com.example.app", "/files", "/data/user/0/com.example.app/files"},
		{"com.example.app", "files", "/data/user/0/com.example.app/files"},
		{"com.example.app", "", "/data/user/0/com.example.app"},
		{"com.example.app", "/databases/x.db", "/data/user/0/com.example.app/databases/x.db"},
		{"com.example.app", "/files/", "/data/user/0/com.example.app/files"},
	}
	for _, c := range cases {
		if got := androidContainerPath(c.bundle, c.rel); got != c.want {
			t.Errorf("androidContainerPath(%q,%q) = %q, want %q", c.bundle, c.rel, got, c.want)
		}
	}
}

func TestAndroidContainerPathCannotLeaveTheContainer(t *testing.T) {
	cases := []struct {
		rel, want string
	}{
		{"/../../../../sdcard", "/data/user/0/com.example.app/sdcard"},
		{"files/../../other.app/x", "/data/user/0/com.example.app/other.app/x"},
		{"..", "/data/user/0/com.example.app"},
	}
	for _, c := range cases {
		if got := androidContainerPath("com.example.app", c.rel); got != c.want {
			t.Errorf("androidContainerPath(%q) = %q, want %q", c.rel, got, c.want)
		}
	}
}

func TestResolveAndroidPathKeepsAbsolutePathWithoutBundle(t *testing.T) {
	if got := resolveAndroidPath("", "/sdcard/Download"); got != "/sdcard/Download" {
		t.Errorf("resolveAndroidPath without bundle = %q, want /sdcard/Download", got)
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
	if !strings.HasPrefix(cmd, "run-as ") {
		t.Errorf("command for a container path = %q, want it to start with 'run-as '", cmd)
	}
}
