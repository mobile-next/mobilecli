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
