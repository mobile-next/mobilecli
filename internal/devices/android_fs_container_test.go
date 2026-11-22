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
		got, err := androidContainerPath(c.bundle, c.rel)
		if err != nil || got != c.want {
			t.Errorf("androidContainerPath(%q,%q) = %q, %v; want %q", c.bundle, c.rel, got, err, c.want)
		}
	}
}

func TestAndroidContainerPathRejectsPathsThatLeaveTheContainer(t *testing.T) {
	for _, rel := range []string{"..", "/../../../../sdcard", "files/../../other.app/x", "/../.."} {
		if got, err := androidContainerPath("com.example.app", rel); err == nil {
			t.Errorf("androidContainerPath(%q) = %q, want an error", rel, got)
		}
	}
}

func TestAndroidContainerPathAllowsDotDotThatStaysInside(t *testing.T) {
	got, err := androidContainerPath("com.example.app", "files/../databases/x.db")
	if err != nil || got != "/data/user/0/com.example.app/databases/x.db" {
		t.Errorf("got %q, %v; want /data/user/0/com.example.app/databases/x.db", got, err)
	}
}

func TestResolveAndroidPathKeepsAbsolutePathWithoutBundle(t *testing.T) {
	if got, err := resolveAndroidPath("", "/sdcard/Download"); err != nil || got != "/sdcard/Download" {
		t.Errorf("resolveAndroidPath without bundle = %q, %v; want /sdcard/Download", got, err)
	}
}

// the container path must live under /data/user/, which is what makes
// buildShellCommand wrap the command in run-as and androidPackageName recover
// the package to run as.
func TestAndroidContainerPathIsRunAsWrapped(t *testing.T) {
	p, err := androidContainerPath("com.example.app", "/files/x")
	if err != nil {
		t.Fatal(err)
	}
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
