package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mobile-next/mobilecli/commands"
)

// Both screen-capture entry points validate fps before any device work, so a
// negative value can never reach the device capture launch. There's no device
// mocking in this package, so success cases (omitted/zero/positive) are
// asserted by confirming they get PAST fps validation — they fail later with
// "error finding device" in this device-less test environment, not with the
// fps error. Only negative should fail with the fps error specifically.
func TestScreenCaptureSessionFPSValidation(t *testing.T) {
	cases := []struct {
		name       string
		fps        int
		wantFPSErr bool
	}{
		{"omitted fps", 0, false},
		{"explicit zero fps (same as omitted)", 0, false},
		{"positive fps", 24, false},
		{"negative fps", -1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params, _ := json.Marshal(commands.ScreenCaptureRequest{Format: "avc", FPS: c.fps})
			_, err := handleScreenCaptureSession(params)
			if err == nil {
				t.Fatalf("expected an error (no device in test env), got nil")
			}
			isFPSErr := strings.Contains(err.Error(), "fps must not be negative")
			if isFPSErr != c.wantFPSErr {
				t.Fatalf("fps=%d: got err=%v, wantFPSErr=%v", c.fps, err, c.wantFPSErr)
			}
		})
	}
}

func TestScreenCaptureFPSValidation(t *testing.T) {
	cases := []struct {
		name       string
		fps        int
		wantFPSErr bool
	}{
		{"omitted fps", 0, false},
		{"explicit zero fps (same as omitted)", 0, false},
		{"positive fps", 24, false},
		{"negative fps", -1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params, _ := json.Marshal(commands.ScreenCaptureRequest{Format: "avc", FPS: c.fps})
			err := handleScreenCapture(nil, httptest.NewRecorder(), params)
			if err == nil {
				t.Fatalf("expected an error (no device in test env), got nil")
			}
			isFPSErr := strings.Contains(err.Error(), "fps must not be negative")
			if isFPSErr != c.wantFPSErr {
				t.Fatalf("fps=%d: got err=%v, wantFPSErr=%v", c.fps, err, c.wantFPSErr)
			}
		})
	}
}
