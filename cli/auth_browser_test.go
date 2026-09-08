package cli

import "testing"

func envWith(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestShouldSkipBrowser(t *testing.T) {
	cases := []struct {
		name     string
		flag     bool
		goos     string
		env      map[string]string
		wantSkip bool
	}{
		{"mac desktop opens browser", false, "darwin", nil, false},
		{"windows desktop opens browser", false, "windows", nil, false},
		{"linux with DISPLAY opens browser", false, "linux", map[string]string{"DISPLAY": ":0"}, false},
		{"linux with WAYLAND_DISPLAY opens browser", false, "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, false},
		{"--no-browser flag skips", true, "darwin", nil, true},
		{"ssh session skips", false, "darwin", map[string]string{"SSH_CONNECTION": "1.2.3.4 22"}, true},
		{"ssh tty skips", false, "darwin", map[string]string{"SSH_TTY": "/dev/pts/0"}, true},
		{"CI skips", false, "darwin", map[string]string{"CI": "true"}, true},
		{"headless linux skips", false, "linux", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipBrowser(tc.flag, tc.goos, envWith(tc.env)); got != tc.wantSkip {
				t.Fatalf("got %v, want %v", got, tc.wantSkip)
			}
		})
	}
}

func TestVerificationURLWithCode(t *testing.T) {
	got := verificationURLWithCode("https://app.mobilenext.ai/login/device", "ABCD-1234")
	want := "https://app.mobilenext.ai/login/device?user_code=ABCD-1234"
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
