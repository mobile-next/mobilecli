package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// FocusRequest represents the parameters for a focus-by-identity command.
type FocusRequest struct {
	DeviceID   string `json:"deviceId"`
	Identifier string `json:"identifier"`
	Label      string `json:"label"`
}

// focuser is implemented by devices that support accessibility-identity focus
// (real Apple TV via the DeviceKit device.io.focus RPC over the tunnel transport).
type focuser interface {
	Focus(identifier, label string) (any, error)
}

// FocusResult wraps the focused element returned by the device.
type FocusResult struct {
	Element any `json:"element"`
}

// FocusCommand drives Siri Remote focus to an element selected by accessibility
// identifier and/or label on the specified device.
func FocusCommand(req FocusRequest) *CommandResponse {
	if req.Identifier == "" && req.Label == "" {
		return NewErrorResponse(fmt.Errorf("at least one of --identifier or --label is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	focusDevice, ok := targetDevice.(focuser)
	if !ok {
		return NewErrorResponse(fmt.Errorf("focus by identity is not supported on device %s", targetDevice.ID()))
	}

	// device.io.focus drives Siri Remote focus and is only wired for tvOS. Guard
	// before starting the agent so a real iPhone fails fast with a clear message
	// instead of starting an agent and then failing at the RPC.
	if targetDevice.Platform() != "tvos" {
		return NewErrorResponse(fmt.Errorf("device.io.focus is only supported on tvOS"))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	element, err := focusDevice.Focus(req.Identifier, req.Label)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to focus element on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(FocusResult{Element: element})
}
