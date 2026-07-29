package commands

import (
	"encoding/json"
	"testing"

	"github.com/mobile-next/mobilecli/devices"
)

// devices.list reports the allocation a remote device belongs to in its provider object
func remoteDevice(id, allocationID string) devices.DeviceInfo {
	provider, _ := json.Marshal(devices.DeviceProvider{Type: "mobilenext", AllocationID: allocationID})
	return devices.DeviceInfo{ID: id, Provider: provider}
}

func TestFindDeviceByAllocation(t *testing.T) {
	allocated := remoteDevice("R83X10DZ2TW", "40ec2cb6-1f40-4772-91d9-2c8820f085d0")
	other := remoteDevice("41270DLJG000R7", "ed66ee3a-c33f-40b2-b569-24ca30b2064a")
	local := devices.DeviceInfo{ID: "Pixel_9_Pro"}

	deviceList := []devices.DeviceInfo{local, other, allocated}

	found := findDeviceByAllocation(deviceList, "40ec2cb6-1f40-4772-91d9-2c8820f085d0")
	if found == nil || found.ID != allocated.ID {
		t.Errorf("expected to find %s, got %v", allocated.ID, found)
	}

	if found := findDeviceByAllocation(deviceList, "no-such-allocation"); found != nil {
		t.Errorf("expected no device for unknown allocation, got %s", found.ID)
	}
}
