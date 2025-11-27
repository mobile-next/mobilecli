package commands

import (
	"fmt"
	"github.com/mobile-next/mobilecli/devices"
)

// DumpUIRequest represents the parameters for dumping UI tree
type DumpUIRequest struct {
	DeviceID string `json:"deviceId"`
	Format   string `json:"format"`
}

// DumpUIResponse represents the response for a dump UI command
type DumpUIResponse struct {
	Elements []devices.ScreenElement `json:"elements,omitempty"`
	RawData  interface{}             `json:"rawData,omitempty"`
}

// DumpUICommand starts an agent and dumps the UI tree from the specified device
func DumpUICommand(req DumpUIRequest) *CommandResponse {
	// Find the target device
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	// Start agent if needed
	err = targetDevice.StartAgent(devices.StartAgentConfig{})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %w", targetDevice.ID(), err))
	}

	var response DumpUIResponse

	// Check if raw format is requested
	if req.Format == "raw" {
		rawData, err := targetDevice.DumpSourceRaw()
		if err != nil {
			return NewErrorResponse(fmt.Errorf("failed to dump raw UI from device %s: %w", targetDevice.ID(), err))
		}

		response = DumpUIResponse{
			RawData: rawData,
		}
	} else {
		// Dump UI tree from the device
		elements, err := targetDevice.DumpSource()
		if err != nil {
			return NewErrorResponse(fmt.Errorf("failed to dump UI from device %s: %w", targetDevice.ID(), err))
		}

		response = DumpUIResponse{
			Elements: elements,
		}
	}

	return NewSuccessResponse(response)
}