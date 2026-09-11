package commands

import (
	"encoding/json"
	"fmt"

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

// PinchRequest represents the parameters for a pinch command. X,Y is the
// center of the pinch; both zero means the center of the screen.
type PinchRequest struct {
	DeviceID  string `json:"deviceId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
	Distance  int    `json:"distance"`
	Duration  int    `json:"duration"`
}

const (
	PinchDirectionIn  = "in"
	PinchDirectionOut = "out"

	defaultPinchDistance   = 200
	defaultPinchDurationMs = 300

	// pinchStartGap is how far from the center each finger touches down (or
	// lifts, for "in"), so the two fingers never share a point.
	pinchStartGap = 30

	// pinchPressHoldMs keeps both fingers still for a few frames before they
	// travel, the way a real pinch begins.
	pinchPressHoldMs = 50
)

// pinchActions lays two fingers on the horizontal line through (x, y), each
// pinchStartGap from the center, and moves them distance pixels away from it
// ("out", zooms in) or toward it ("in", zooms out) over duration milliseconds.
// Each finger is one complete, contiguous pointer sequence with its own Button,
// which is what devicekit.ConvertActions expects; the agents run both fingers
// at the same time.
func pinchActions(x, y int, direction string, distance, duration int) ([]devicekit.TapAction, error) {
	if distance <= 0 {
		distance = defaultPinchDistance
	}
	if duration <= 0 {
		duration = defaultPinchDurationMs
	}

	near, far := pinchStartGap, pinchStartGap+distance
	var startOffset, endOffset int
	switch direction {
	case PinchDirectionOut:
		startOffset, endOffset = near, far
	case PinchDirectionIn:
		startOffset, endOffset = far, near
	default:
		return nil, fmt.Errorf("direction must be %q or %q, got %q", PinchDirectionIn, PinchDirectionOut, direction)
	}

	if x-far < 0 || y < 0 {
		return nil, fmt.Errorf("pinch centered at (%d,%d) with distance %d does not fit on screen: the left finger would reach x=%d", x, y, distance, x-far)
	}

	var actions []devicekit.TapAction
	for finger, side := range []int{-1, +1} {
		actions = append(actions,
			devicekit.TapAction{Type: "pointerMove", X: x + side*startOffset, Y: y, Button: finger},
			devicekit.TapAction{Type: "pointerDown", Button: finger},
			devicekit.TapAction{Type: "pause", Duration: pinchPressHoldMs, Button: finger},
			devicekit.TapAction{Type: "pointerMove", X: x + side*endOffset, Y: y, Duration: duration, Button: finger},
			devicekit.TapAction{Type: "pointerUp", Button: finger},
		)
	}
	return actions, nil
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
// does, and returns the center of the element matching ref ("@e5").
// Refs are positional against a fresh dump; there is no staleness tracking.
func resolveRefTapPoint(device devices.ControllableDevice, ref string) (int, int, error) {
	elements, err := device.DumpSource(devices.DumpOptions{})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to dump UI to resolve ref %s: %w", ref, err)
	}

	types.AttachRefs(elements)
	element := findElementByRef(elements, ref)
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

// PinchCommand performs a two-finger pinch on the specified device
func PinchCommand(req PinchRequest) *CommandResponse {
	if req.Direction != PinchDirectionIn && req.Direction != PinchDirectionOut {
		return NewErrorResponse(fmt.Errorf("direction must be %q or %q, got %q", PinchDirectionIn, PinchDirectionOut, req.Direction))
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
	if x == 0 && y == 0 {
		x, y, err = screenCenter(targetDevice)
		if err != nil {
			return NewErrorResponse(err)
		}
	}

	actions, err := pinchActions(x, y, req.Direction, req.Distance, req.Duration)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = targetDevice.Gesture(actions)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to pinch on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Pinched %s on device %s around (%d,%d)", req.Direction, targetDevice.ID(), x, y),
	})
}

func screenCenter(device devices.ControllableDevice) (int, int, error) {
	info, err := device.Info()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get screen size of device %s: %v", device.ID(), err)
	}
	if info.ScreenSize == nil {
		return 0, 0, fmt.Errorf("device %s did not report a screen size; pass x,y explicitly", device.ID())
	}
	return info.ScreenSize.Width / 2, info.ScreenSize.Height / 2, nil
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
