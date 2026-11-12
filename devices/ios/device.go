package ios

import (
	"fmt"

	goios "github.com/danielpaulus/go-ios/ios"
)

const usbConnectionType = "USB"

// GetDevice looks up a device by UDID, preferring its USB entry. usbmuxd lists a device that is
// also paired over Wi-Fi twice, and go-ios returns whichever entry comes first. Service connections
// through the network entry can fail (usbmuxd error code 2), which breaks port forwarding to the
// on-device agent while the device is plugged in.
func GetDevice(udid string) (goios.DeviceEntry, error) {
	list, err := goios.ListDevices()
	if err != nil {
		return goios.DeviceEntry{}, err
	}

	device, ok := pickDeviceEntry(list.DeviceList, udid)
	if !ok {
		return goios.DeviceEntry{}, fmt.Errorf("device '%s' not found, is it attached to the machine?", udid)
	}
	return device, nil
}

// pickDeviceEntry returns the entry for udid, taking the USB entry over a network one.
func pickDeviceEntry(entries []goios.DeviceEntry, udid string) (goios.DeviceEntry, bool) {
	var found goios.DeviceEntry
	ok := false
	for _, entry := range entries {
		if entry.Properties.SerialNumber != udid {
			continue
		}
		if entry.Properties.ConnectionType == usbConnectionType {
			return entry, true
		}
		if !ok {
			found, ok = entry, true
		}
	}
	return found, ok
}
