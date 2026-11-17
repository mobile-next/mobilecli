package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// setConfiguration must reject a non-positive bitrate before doing any device
// work, so a malformed request can never reach the encoder.
func TestSetConfigurationRejectsNonPositiveBitrate(t *testing.T) {
	for _, bitrate := range []int{0, -1} {
		params, _ := json.Marshal(screenCaptureSetConfigRequest{DeviceID: "dev123", Bitrate: bitrate})
		if _, err := handleScreenCaptureSetConfiguration(params); err == nil {
			t.Fatalf("expected error for bitrate=%d, got nil", bitrate)
		} else if !strings.Contains(err.Error(), "bitrate must be positive") {
			t.Fatalf("expected 'bitrate must be positive' for bitrate=%d, got %v", bitrate, err)
		}
	}
}

// Malformed params should be rejected cleanly rather than panicking.
func TestSetConfigurationRejectsInvalidParams(t *testing.T) {
	if _, err := handleScreenCaptureSetConfiguration(json.RawMessage(`{"bitrate":"nope"}`)); err == nil {
		t.Fatal("expected error for invalid params, got nil")
	}
}
