package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

func InfoCommand(deviceID string) (*devices.FullDeviceInfo, error) {
	targetDevice, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %v", err)
	}

	info, err := targetDevice.Info()
	if err != nil {
		return nil, fmt.Errorf("error getting device info: %v", err)
	}

	return info, nil
}
