package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/internal/devices"
)

// DeviceInfoResponse represents the response for a device info command
type DeviceInfoResponse struct {
	Device *devices.FullDeviceInfo `json:"device"`
}

func InfoCommand(deviceID string) *CommandResponse {
	targetDevice, err := findDeviceForInfo(deviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	info, err := targetDevice.Info()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error getting device info: %v", err))
	}

	response := DeviceInfoResponse{
		Device: info,
	}

	return NewSuccessResponse(response)
}

// findDeviceForInfo skips the agent on real iOS devices: their screen size
// comes from lockdown, so device info works without an installed agent.
func findDeviceForInfo(deviceID string) (devices.ControllableDevice, error) {
	targetDevice, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %w", err)
	}
	if _, isRealIOS := targetDevice.(*devices.IOSDevice); isRealIOS {
		return targetDevice, nil
	}
	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start agent on device %s: %w", targetDevice.ID(), err)
	}
	return targetDevice, nil
}
