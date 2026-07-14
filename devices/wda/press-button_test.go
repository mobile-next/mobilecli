package wda

import (
	"strings"
	"testing"
)

func TestValidateButtonForPlatform(t *testing.T) {
	cases := []struct {
		name     string
		platform string
		button   string
		wantErr  bool
		errFrag  string
	}{
		// tvOS accepts all seven Siri Remote buttons.
		{"tvos up", "tvos", "UP", false, ""},
		{"tvos down", "tvos", "DOWN", false, ""},
		{"tvos left", "tvos", "LEFT", false, ""},
		{"tvos right", "tvos", "RIGHT", false, ""},
		{"tvos select", "tvos", "SELECT", false, ""},
		{"tvos menu", "tvos", "MENU", false, ""},
		{"tvos play_pause", "tvos", "PLAY_PAUSE", false, ""},
		// tvOS rejects iPhone-only buttons explicitly.
		{"tvos home", "tvos", "HOME", true, "unsupported on tvOS"},
		{"tvos lock", "tvos", "LOCK", true, "unsupported on tvOS"},
		{"tvos volume_up", "tvos", "VOLUME_UP", true, "unsupported on tvOS"},
		{"tvos volume_down", "tvos", "VOLUME_DOWN", true, "unsupported on tvOS"},
		// iOS accepts its hardware buttons.
		{"ios home", "ios", "HOME", false, ""},
		{"ios volume_up", "ios", "VOLUME_UP", false, ""},
		// iOS rejects Siri Remote buttons.
		{"ios select", "ios", "SELECT", true, "unsupported on ios"},
		{"ios menu", "ios", "MENU", true, "unsupported on ios"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateButtonForPlatform(c.platform, c.button)
			if c.wantErr && err == nil {
				t.Fatalf("expected error for %s/%s, got nil", c.platform, c.button)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error for %s/%s: %v", c.platform, c.button, err)
			}
			if c.wantErr && c.errFrag != "" && !strings.Contains(err.Error(), c.errFrag) {
				t.Errorf("error %q does not contain %q", err.Error(), c.errFrag)
			}
		})
	}
}
