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

// DumpUIResponse represents the response for a raw-format dump UI command.
type DumpUIResponse struct {
	RawData any `json:"rawData,omitempty"`
}

// DumpUIElementsResponse is the response for a (default) json-format dump.
// `elements` is intentionally not omitempty and is always populated (with an
// empty array when the screen has no labelled elements) so clients can rely on
// the field existing instead of breaking on a missing key.
type DumpUIElementsResponse struct {
	Elements []devices.ScreenElement `json:"elements"`
}

// DumpUICommand starts an agent and dumps the UI tree from the specified device
func DumpUICommand(req DumpUIRequest) *CommandResponse {
	// Find the target device
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	// Start agent if needed
	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %w", targetDevice.ID(), err))
	}

	// Check if raw format is requested
	if req.Format == "raw" {
		rawData, err := targetDevice.DumpSourceRaw()
		if err != nil {
			return NewErrorResponse(fmt.Errorf("failed to dump raw UI from device %s: %w", targetDevice.ID(), err))
		}

		return NewSuccessResponse(DumpUIResponse{RawData: rawData})
	}

	// Dump UI tree from the device
	elements, err := targetDevice.DumpSource()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to dump UI from device %s: %w", targetDevice.ID(), err))
	}

	// Always return a well-formed `elements` array, even when empty: a screen
	// with no labelled elements (or a mid-transition dump) otherwise produced a
	// response with the field omitted, breaking clients that read `elements`.
	if elements == nil {
		elements = []devices.ScreenElement{}
	}

	return NewSuccessResponse(DumpUIElementsResponse{Elements: elements})
}
