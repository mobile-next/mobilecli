package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// rejects bad input before any device lookup, so these run without a device.
func TestSetConfigurationRejectsNonPositiveBitrate(t *testing.T) {
	_, err := handleScreenCaptureSetConfiguration(json.RawMessage(`{"deviceId":"x","bitrate":0}`))
	if err == nil || !strings.Contains(err.Error(), "bitrate must be positive") {
		t.Fatalf("expected positive-bitrate error, got: %v", err)
	}
}

func TestSetConfigurationRejectsInvalidJSON(t *testing.T) {
	_, err := handleScreenCaptureSetConfiguration(json.RawMessage(`not json`))
	if err == nil || !strings.Contains(err.Error(), "invalid parameters") {
		t.Fatalf("expected invalid-parameters error, got: %v", err)
	}
}
