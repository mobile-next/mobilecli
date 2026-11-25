package ios

import (
	"testing"

	goios "github.com/danielpaulus/go-ios/ios"
)

func entry(id int, udid, connection string) goios.DeviceEntry {
	return goios.DeviceEntry{
		DeviceID:   id,
		Properties: goios.DeviceProperties{SerialNumber: udid, ConnectionType: connection},
	}
}

func TestPickDeviceEntryPrefersUSB(t *testing.T) {
	entries := []goios.DeviceEntry{
		entry(8, "udid-1", "Network"),
		entry(7, "udid-1", "USB"),
		entry(2, "udid-2", "Network"),
	}

	got, ok := pickDeviceEntry(entries, "udid-1")
	if !ok || got.DeviceID != 7 {
		t.Fatalf("expected USB entry 7, got %d (found=%v)", got.DeviceID, ok)
	}
}

func TestPickDeviceEntryFallsBackToNetwork(t *testing.T) {
	entries := []goios.DeviceEntry{
		entry(8, "udid-1", "USB"),
		entry(2, "udid-2", "Network"),
	}

	got, ok := pickDeviceEntry(entries, "udid-2")
	if !ok || got.DeviceID != 2 {
		t.Fatalf("expected network entry 2, got %d (found=%v)", got.DeviceID, ok)
	}
}

func TestPickDeviceEntryNotFound(t *testing.T) {
	if _, ok := pickDeviceEntry([]goios.DeviceEntry{entry(1, "udid-1", "USB")}, "missing"); ok {
		t.Fatal("expected no entry for unknown udid")
	}
}

func TestUSBConnection(t *testing.T) {
	tests := []struct {
		name      string
		entries   []goios.DeviceEntry
		wantUSB   bool
		wantFound bool
	}{
		{"usb only", []goios.DeviceEntry{entry(7, "udid-1", "USB")}, true, true},
		{"network only", []goios.DeviceEntry{entry(8, "udid-1", "Network")}, false, true},
		{"network listed before usb", []goios.DeviceEntry{entry(8, "udid-1", "Network"), entry(7, "udid-1", "USB")}, true, true},
		{"usb of another device", []goios.DeviceEntry{entry(7, "udid-2", "USB"), entry(8, "udid-1", "Network")}, false, true},
		{"not listed", []goios.DeviceEntry{entry(7, "udid-2", "USB")}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usb, found := usbConnection(tt.entries, "udid-1")
			if usb != tt.wantUSB || found != tt.wantFound {
				t.Fatalf("usbConnection = (%v, %v), want (%v, %v)", usb, found, tt.wantUSB, tt.wantFound)
			}
		})
	}
}
