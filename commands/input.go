package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/devices/devicekit"
	"github.com/mobile-next/mobilecli/types"
)

// TapRequest represents the parameters for a tap command. Either X,Y or Ref
// ("@e5", from the latest dump) must be set; Ref wins when both are set.
type TapRequest struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Ref      string `json:"ref,omitempty"`
}

// LongPressRequest represents the parameters for a long press command
type LongPressRequest struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Duration int    `json:"duration"`
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
	DeviceID string `json:"deviceId"`
	Actions  []any  `json:"actions"`
}

// SwipeRequest represents the parameters for a swipe command
type SwipeRequest struct {
	DeviceID string `json:"deviceId"`
	X1       int    `json:"x1"`
	Y1       int    `json:"y1"`
	X2       int    `json:"x2"`
	Y2       int    `json:"y2"`
	Duration int    `json:"duration"`
}

// TapCommand performs a tap operation on the specified device
func TapCommand(req TapRequest) *CommandResponse {
	if req.Ref == "" && (req.X < 0 || req.Y < 0) {
		return NewErrorResponse(fmt.Errorf("x and y coordinates must be non-negative, got x=%d, y=%d", req.X, req.Y))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	x, y := req.X, req.Y
	if req.Ref != "" {
		x, y, err = resolveRefTapPoint(targetDevice, req.Ref)
		if err != nil {
			return NewErrorResponse(err)
		}
	}

	err = targetDevice.Tap(x, y)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to tap on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Tapped on device %s at (%d,%d)", targetDevice.ID(), x, y),
	})
}

// resolveRefTapPoint re-dumps the UI tree, numbers it exactly like "dump ui"
// does, and returns the center of the element matching ref ("@e5" or "e5").
// Refs are positional against a fresh dump; there is no staleness tracking.
func resolveRefTapPoint(device devices.ControllableDevice, ref string) (int, int, error) {
	elements, err := device.DumpSource()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to dump UI to resolve ref %s: %w", ref, err)
	}

	types.AttachRefs(elements)
	element := findElementByRef(elements, strings.TrimPrefix(ref, "@"))
	if element == nil {
		return 0, 0, fmt.Errorf("ref %s not found on current screen; refs come from the latest 'dump ui'", ref)
	}

	return element.Rect.X + element.Rect.Width/2, element.Rect.Y + element.Rect.Height/2, nil
}

func findElementByRef(elements []types.ScreenElement, ref string) *types.ScreenElement {
	for i := range elements {
		if elements[i].Ref == ref {
			return &elements[i]
		}
		if found := findElementByRef(elements[i].Children, ref); found != nil {
			return found
		}
	}
	return nil
}

// LongPressCommand performs a long press operation on the specified device
func LongPressCommand(req LongPressRequest) *CommandResponse {
	if req.X < 0 || req.Y < 0 {
		return NewErrorResponse(fmt.Errorf("x and y coordinates must be non-negative, got x=%d, y=%d", req.X, req.Y))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.LongPress(req.X, req.Y, req.Duration)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to long press on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Long pressed on device %s at (%d,%d) for %dms", targetDevice.ID(), req.X, req.Y, req.Duration),
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

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.SendKeys(req.Text)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to send text to device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Sent text to device %s", targetDevice.ID()),
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

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.PressButton(req.Button)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to press button on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Pressed button '%s' on device %s", req.Button, targetDevice.ID()),
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

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	// Convert []any to []devicekit.TapAction
	tapActions := make([]devicekit.TapAction, len(req.Actions))
	for i, action := range req.Actions {
		actionBytes, err := json.Marshal(action)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("failed to marshal action at index %d: %v", i, err))
		}

		var tapAction devicekit.TapAction
		if err := json.Unmarshal(actionBytes, &tapAction); err != nil {
			return NewErrorResponse(fmt.Errorf("failed to unmarshal action at index %d: %v", i, err))
		}
		tapActions[i] = tapAction
	}

	err = targetDevice.Gesture(tapActions)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to perform gesture on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Performed gesture on device %s with %d actions", targetDevice.ID(), len(req.Actions)),
	})
}

// SwipeCommand performs a swipe operation on the specified device
func SwipeCommand(req SwipeRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	err = targetDevice.Swipe(req.X1, req.Y1, req.X2, req.Y2, req.Duration)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to swipe on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Swiped on device %s from (%d,%d) to (%d,%d)", targetDevice.ID(), req.X1, req.Y1, req.X2, req.Y2),
	})
}
