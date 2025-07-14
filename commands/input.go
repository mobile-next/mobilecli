package commands

import (
	"fmt"
)

// TapRequest represents the parameters for a tap command
type TapRequest struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

// TextRequest represents the parameters for a text input command
type TextRequest struct {
	DeviceID string `json:"deviceId"`
	Text     string `json:"text"`
}

// ButtonRequest represents the parameters for a button press command
type ButtonRequest struct {
	DeviceID string `json:"deviceId"`
	Button   string `json:"button"`
}

// GestureRequest represents the parameters for a gesture command
type GestureRequest struct {
	DeviceID string        `json:"deviceId"`
	Actions  []interface{} `json:"actions"`
}

// TapCommand performs a tap operation on the specified device
func TapCommand(req TapRequest) *CommandResponse {
	if req.X < 0 || req.Y < 0 {
		return NewErrorResponse(fmt.Errorf("x and y coordinates must be non-negative, got x=%d, y=%d", req.X, req.Y))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.Tap(req.X, req.Y)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to tap on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Tapped on device %s at (%d,%d)", targetDevice.ID(), req.X, req.Y),
	})
}

// TextCommand sends text input to the specified device
func TextCommand(req TextRequest) *CommandResponse {
	if req.Text == "" {
		return NewErrorResponse(fmt.Errorf("text is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.SendKeys(req.Text)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to send text to device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Sent text to device %s", targetDevice.ID()),
	})
}

// ButtonCommand presses a hardware button on the specified device
func ButtonCommand(req ButtonRequest) *CommandResponse {
	if req.Button == "" {
		return NewErrorResponse(fmt.Errorf("button name is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.PressButton(req.Button)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to press button on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Pressed button '%s' on device %s", req.Button, targetDevice.ID()),
	})
}

// GestureCommand performs a gesture operation on the specified device
func GestureCommand(req GestureRequest) *CommandResponse {
	if len(req.Actions) == 0 {
		return NewErrorResponse(fmt.Errorf("actions array is required and cannot be empty"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.Gesture(req.Actions)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to perform gesture on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Performed gesture on device %s with %d actions", targetDevice.ID(), len(req.Actions)),
	})
}
