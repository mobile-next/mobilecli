package server

import (
	"testing"
	"time"
)

// Slow, device-side RPCs must get an extended write deadline. Without it they run
// under the default WriteTimeout, and any call that outlives it has its connection
// closed mid-response — the caller then sees an opaque EOF instead of the result.
// device.apps.install was missing from this set, which caused real install failures
// on devices where the install took longer than the default WriteTimeout.
func TestExtendedWriteDeadlineForSlowMethods(t *testing.T) {
	slowMethods := map[string]time.Duration{
		"device.boot":              3 * time.Minute,
		"device.apps.install":      3 * time.Minute,
		"device.apps.uninstall":    3 * time.Minute,
		"device.screenrecord.stop": 35 * time.Second,
	}

	for method, want := range slowMethods {
		got, ok := extendedWriteDeadline(method)
		if !ok {
			t.Errorf("%s: expected an extended write deadline, got none", method)
			continue
		}
		if got != want {
			t.Errorf("%s: expected deadline %s, got %s", method, want, got)
		}
	}
}

// Fast methods run under the default WriteTimeout and must not be extended.
func TestNoExtendedWriteDeadlineForFastMethods(t *testing.T) {
	for _, method := range []string{"device.info", "device.screenshot", "device.io.tap", ""} {
		if d, ok := extendedWriteDeadline(method); ok {
			t.Errorf("%q: expected no extension, got %s", method, d)
		}
	}
}
