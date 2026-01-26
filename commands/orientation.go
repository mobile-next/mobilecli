package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// OrientationGetRequest represents the request for getting device orientation
type OrientationGetRequest struct {
	DeviceID string `json:"deviceId"`
}

// OrientationSetRequest represents the request for setting device orientation
type OrientationSetRequest struct {
	DeviceID    string `json:"deviceId"`
	Orientation string `json:"orientation"`
}

// OrientationResponse represents the response containing orientation information
type OrientationResponse struct {
	Orientation string `json:"orientation"`
}

// OrientationGetCommand gets the current device orientation
func OrientationGetCommand(req OrientationGetRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	// start agent if needed
	err = device.StartAgent(devices.StartAgentConfig{
		Registry: GetRegistry(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", device.ID(), err))
	}

	orientation, err := device.GetOrientation()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to get orientation: %v", err))
	}

	response := OrientationResponse{
		Orientation: orientation,
	}

	return NewSuccessResponse(response)
}

// OrientationSetCommand sets the device orientation
func OrientationSetCommand(req OrientationSetRequest) *CommandResponse {
	// validate orientation value
	if req.Orientation != "portrait" && req.Orientation != "landscape" {
		return NewErrorResponse(fmt.Errorf("invalid orientation value '%s', must be 'portrait' or 'landscape'", req.Orientation))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	// start agent if needed
	err = device.StartAgent(devices.StartAgentConfig{
		Registry: GetRegistry(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", device.ID(), err))
	}

	err = device.SetOrientation(req.Orientation)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to set orientation: %v", err))
	}

	response := OrientationResponse{
		Orientation: req.Orientation,
	}

	return NewSuccessResponse(response)
}