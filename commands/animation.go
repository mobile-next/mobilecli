package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// AnimationScalesGetRequest represents the request for getting animation scales
type AnimationScalesGetRequest struct {
	DeviceID string `json:"deviceId"`
}

// AnimationScalesSetRequest represents the request for setting animation scales
type AnimationScalesSetRequest struct {
	DeviceID   string  `json:"deviceId"`
	Window     float64 `json:"window"`
	Transition float64 `json:"transition"`
	Animator   float64 `json:"animator"`
}

// AnimationScalesResponse holds the three Android global animation scale values
type AnimationScalesResponse struct {
	Window     float64 `json:"window"`
	Transition float64 `json:"transition"`
	Animator   float64 `json:"animator"`
}

// AnimationScalesGetCommand gets the current animation scale values from the device
func AnimationScalesGetCommand(req AnimationScalesGetRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = device.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", device.ID(), err))
	}

	scales, err := device.GetAnimationScales()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to get animation scales: %v", err))
	}

	return NewSuccessResponse(AnimationScalesResponse{
		Window:     scales.Window,
		Transition: scales.Transition,
		Animator:   scales.Animator,
	})
}

// AnimationScalesSetCommand sets the animation scale values on the device
func AnimationScalesSetCommand(req AnimationScalesSetRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = device.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", device.ID(), err))
	}

	err = device.SetAnimationScales(devices.AnimationScales{
		Window:     req.Window,
		Transition: req.Transition,
		Animator:   req.Animator,
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to set animation scales: %v", err))
	}

	return NewSuccessResponse(AnimationScalesResponse{
		Window:     req.Window,
		Transition: req.Transition,
		Animator:   req.Animator,
	})
}
