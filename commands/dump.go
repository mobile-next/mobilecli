package commands

import (
	"fmt"
	"github.com/mobile-next/mobilecli/devices"
)

// DumpUIRequest represents the parameters for dumping UI tree
type DumpUIRequest struct {
	DeviceID string `json:"deviceId"`
}

// DumpUIResponse represents the response for a dump UI command
type DumpUIResponse struct {
	Elements []devices.ScreenElement `json:"elements"`
}

// DumpUICommand starts an agent and dumps the UI tree from the specified device
func DumpUICommand(req DumpUIRequest) *CommandResponse {
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

	// Dump UI tree from the device
	elements, err := targetDevice.DumpSource()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to dump UI from device %s: %v", targetDevice.ID(), err))
	}

	response := DumpUIResponse{
		Elements: elements,
	}

	return NewSuccessResponse(response)
}