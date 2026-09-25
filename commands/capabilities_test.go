package commands

import (
	"strings"
	"testing"

	"github.com/mobile-next/mobilecli/devices"
)

// deviceWithoutFileAccess satisfies ControllableDevice but none of the
// optional capability interfaces.
type deviceWithoutFileAccess struct {
	devices.ControllableDevice
}

func (deviceWithoutFileAccess) Platform() string   { return "test" }
func (deviceWithoutFileAccess) DeviceType() string { return "fake" }

func TestRequireCapabilityRejectsDeviceWithoutIt(t *testing.T) {
	_, err := requireCapability[devices.FileSystem](deviceWithoutFileAccess{}, "file access")
	if err == nil {
		t.Fatal("expected an error for a device without file access")
	}
	if !strings.Contains(err.Error(), "file access is not supported on test fake devices") {
		t.Fatalf("unexpected error: %v", err)
	}
}
