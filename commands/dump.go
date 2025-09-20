package commands

import (
	"fmt"
	"github.com/mobile-next/mobilecli/devices"
)

// DumpSourceRequest represents the parameters for dumping source tree
type DumpSourceRequest struct {
	DeviceID string `json:"deviceId"`
}

// DumpSourceResponse represents the response for a dump source command
type DumpSourceResponse struct {
	Elements []devices.ScreenElement `json:"elements"`
}

// DumpSourceCommand starts an agent and dumps the source tree from the specified device
func DumpSourceCommand(req DumpSourceRequest) *CommandResponse {
	// Find the target device
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	// Start agent if needed
	err = targetDevice.StartAgent()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	// Dump source tree from the device
	elements, err := targetDevice.DumpSource()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to dump source from device %s: %v", targetDevice.ID(), err))
	}

	response := DumpSourceResponse{
		Elements: elements,
	}

	return NewSuccessResponse(response)
}