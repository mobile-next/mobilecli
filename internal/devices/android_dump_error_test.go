package devices

import (
	"errors"
	"testing"
)

// When the device server dump fails and uiautomator fails too, the error has to
// name both: the uiautomator text alone hid an adb forward failure on WSL2.
func TestViewTreeDumpErrorNamesBothPaths(t *testing.T) {
	deviceServerErr := errors.New("adb forward: cannot bind listener")
	uiautomatorErr := errors.New("no XML content found in uiautomator dump")

	err := viewTreeDumpError(uiautomatorErr, deviceServerErr)

	if !errors.Is(err, uiautomatorErr) {
		t.Fatalf("uiautomator error is not wrapped: %v", err)
	}
	if !errors.Is(err, deviceServerErr) {
		t.Fatalf("device server error is not wrapped: %v", err)
	}
}

// Without a device server error the message stays what it always was.
func TestViewTreeDumpErrorWithoutDeviceServerError(t *testing.T) {
	uiautomatorErr := errors.New("no XML content found in uiautomator dump")

	err := viewTreeDumpError(uiautomatorErr, nil)

	if err.Error() != "failed to get view tree dump: no XML content found in uiautomator dump" {
		t.Fatalf("unexpected message: %v", err)
	}
	if !errors.Is(err, uiautomatorErr) {
		t.Fatalf("uiautomator error is not wrapped: %v", err)
	}
}
