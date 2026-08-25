package devices

import "testing"

func Test_androidParseLsLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		dirPath  string
		wantName string
		wantPath string
		wantSize int64
		wantDir  bool
		wantNil  bool
	}{
		{
			name:     "regular file",
			line:     "-rw-r--r-- 1 root root 1024 2024-01-15 12:34 file.txt",
			dirPath:  "/sdcard",
			wantName: "file.txt",
			wantPath: "/sdcard/file.txt",
			wantSize: 1024,
		},
		{
			name:     "directory",
			line:     "drwxr-xr-x 2 root root 4096 2024-01-15 12:34 Download",
			dirPath:  "/sdcard",
			wantName: "Download",
			wantPath: "/sdcard/Download",
			wantSize: 0,
			wantDir:  true,
		},
		{
			name:     "symlink with arrow target stripped",
			line:     "lrwxrwxrwx 1 root root 21 2024-01-15 12:34 sdcard -> /storage/self/primary",
			dirPath:  "/",
			wantName: "sdcard",
			wantPath: "/sdcard",
			wantSize: 21,
		},
		{
			name:     "character device with major,minor",
			line:     "crw-rw-rw- 1 root root 1, 3 2024-01-15 12:34 null",
			dirPath:  "/dev",
			wantName: "null",
			wantPath: "/dev/null",
			wantSize: 0,
		},
		{
			name:     "block device with no space after comma",
			line:     "brw------- 1 root root 7,0 2024-01-15 12:34 loop0",
			dirPath:  "/dev/block",
			wantName: "loop0",
			wantPath: "/dev/block/loop0",
			wantSize: 0,
		},
		{
			name:     "filename with spaces preserved",
			line:     "-rw-r--r-- 1 root root 17 2024-01-15 12:34 my  cool file.txt",
			dirPath:  "/sdcard",
			wantName: "my  cool file.txt",
			wantPath: "/sdcard/my  cool file.txt",
			wantSize: 17,
		},
		{
			name:     "absolute path in name (single-file ls)",
			line:     "-rw-r--r-- 1 root root 5 2024-01-15 12:34 /sdcard/Download/a.txt",
			dirPath:  "/sdcard/Download/a.txt",
			wantName: "a.txt",
			wantPath: "/sdcard/Download/a.txt",
			wantSize: 5,
		},
		{
			name:    "total line",
			line:    "total 24",
			wantNil: true,
		},
		{
			name:    "empty line",
			line:    "",
			wantNil: true,
		},
		{
			name:    "garbage line",
			line:    "this is not ls output",
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := androidParseLsLine(tc.line, tc.dirPath)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected entry, got nil")
			}
			if got.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", got.Name, tc.wantName)
			}
			if got.Path != tc.wantPath {
				t.Errorf("Path: got %q, want %q", got.Path, tc.wantPath)
			}
			if got.Size != tc.wantSize {
				t.Errorf("Size: got %d, want %d", got.Size, tc.wantSize)
			}
			if got.IsDir != tc.wantDir {
				t.Errorf("IsDir: got %v, want %v", got.IsDir, tc.wantDir)
			}
		})
	}
}

func Test_androidParseLsOutput_filtersDotEntries(t *testing.T) {
	output := `total 12
drwxr-xr-x 3 root root 4096 2024-01-15 12:34 .
drwxr-xr-x 9 root root 4096 2024-01-15 12:34 ..
-rw-r--r-- 1 root root 7 2024-01-15 12:34 hello.txt
`
	entries := androidParseLsOutput(output, "/sdcard/x")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Name != "hello.txt" {
		t.Errorf("got %q, want %q", entries[0].Name, "hello.txt")
	}
}

func Test_buildShellCommand_quotesUntrustedInput(t *testing.T) {
	d := &AndroidDevice{}

	cmd, err := d.buildShellCommand("/sdcard/x", "cat", "/sdcard/a;rm -rf /")
	if err != nil {
		t.Fatal(err)
	}
	// the dangerous path must be quoted so the device shell sees it as a single arg
	want := `cat '/sdcard/a;rm -rf /'`
	if cmd != want {
		t.Errorf("got %q, want %q", cmd, want)
	}

	cmd, err = d.buildShellCommand("/data/user/0/com.evil;reboot/files/x", "cat", "/data/user/0/com.evil;reboot/files/x")
	if err != nil {
		t.Fatal(err)
	}
	// the package name `com.evil;reboot` must be quoted inside run-as
	wantPrefix := `run-as 'com.evil;reboot' cat `
	if len(cmd) < len(wantPrefix) || cmd[:len(wantPrefix)] != wantPrefix {
		t.Errorf("got %q, want prefix %q", cmd, wantPrefix)
	}
}
