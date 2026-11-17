package server

import (
	"encoding/json"
	"strings"
	"testing"
)

// A missing text must not be read as "", which would clear the field.
func TestHandleIoFillRejectsMissingText(t *testing.T) {
	_, err := handleIoFill(json.RawMessage(`{"deviceId":"emulator-5554"}`))
	if err == nil || !strings.Contains(err.Error(), "'text' is required") {
		t.Fatalf("expected a missing text error, got %v", err)
	}
}

func TestHandleIoFillRejectsNonStringText(t *testing.T) {
	_, err := handleIoFill(json.RawMessage(`{"deviceId":"emulator-5554","text":42}`))
	if err == nil || !strings.Contains(err.Error(), "invalid parameters") {
		t.Fatalf("expected an invalid parameters error, got %v", err)
	}
}

// An explicit "" is valid and gets past validation; it then fails only because the device isn't there.
func TestHandleIoFillAcceptsEmptyText(t *testing.T) {
	_, err := handleIoFill(json.RawMessage(`{"deviceId":"no-such-device","text":""}`))
	if err != nil && (strings.Contains(err.Error(), "'text' is required") || strings.Contains(err.Error(), "invalid parameters")) {
		t.Fatalf("expected an empty text to pass validation, got %v", err)
	}
}
