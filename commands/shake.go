package commands

import "fmt"

// shaker is implemented by devices that can simulate a shake (simulators and emulators)
type shaker interface {
	Shake() error
}

// ShakeCommand simulates a shake gesture on the specified device
func ShakeCommand(req DeviceIDRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	device, ok := targetDevice.(shaker)
	if !ok {
		return NewErrorResponse(fmt.Errorf("device %s does not support shake", targetDevice.ID()))
	}

	if err := device.Shake(); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to shake device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Shook device %s", targetDevice.ID()),
	})
}
