package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/internal/devices"
	"github.com/mobile-next/mobilecli/internal/devices/devicekit"
	"github.com/mobile-next/mobilecli/internal/types"
)

// TapRequest represents the parameters for a tap command. Either X,Y or Ref
// ("@e5", from the latest dump) must be set; Ref wins when both are set.
type TapRequest struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Ref      string `json:"ref,omitempty"`
}

// DoubleTapRequest represents the parameters for a double tap command. Either
// X,Y or Ref ("@e5", from the latest dump) must be set; Ref wins when both are.
type DoubleTapRequest struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Ref      string `json:"ref,omitempty"`
}

// LongPressRequest represents the parameters for a long press command. Either
// X,Y or Ref ("@e5", from the latest dump) must be set; Ref wins when both are.
type LongPressRequest struct {
	DeviceID string `json:"deviceId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Duration int    `json:"duration"`
	Ref      string `json:"ref,omitempty"`
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

// DragPoint is one end of a drag: either X,Y or Ref ("@e5", from the latest
// dump); Ref wins when both are set.
type DragPoint struct {
	X   int    `json:"x"`
	Y   int    `json:"y"`
	Ref string `json:"ref,omitempty"`
}

// DragRequest represents the parameters for a drag command: press at From,
// stay there for HoldMs, travel to To over DurationMs, then lift.
type DragRequest struct {
	DeviceID   string    `json:"deviceId"`
	From       DragPoint `json:"from"`
	To         DragPoint `json:"to"`
	HoldMs     int       `json:"holdMs"`
	DurationMs int       `json:"durationMs"`
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
	// doubleTapHoldMs is how long each of the two taps stays down.
	doubleTapHoldMs = 50

	// doubleTapGapMs is how long the screen is left untouched between the two
	// taps: long enough to read as two taps, well inside the double-tap window.
	doubleTapGapMs = 150
)

// DefaultDragDurationMs is how long a drag travels when no duration is given.
const DefaultDragDurationMs = 800

const (
	PinchDirectionIn  = "in"
	PinchDirectionOut = "out"

	defaultPinchDurationMs = 300

	// minPinchDistance is the smallest travel an auto-scaled pinch uses, so a
	// pinch centered close to the left edge still moves a little rather than
	// collapsing to nothing.
	minPinchDistance = 20

	// pinchStartGap is how far from the center each finger touches down (or
	// lifts, for "in"), so the two fingers never share a point.
	pinchStartGap = 30

	// pinchPressHoldMs keeps both fingers still for a few frames before they
	// travel, the way a real pinch begins.
	pinchPressHoldMs = 50
)

// autoPinchDistance picks a travel that keeps the inner finger on-screen when no
// distance is given. The binding constraint is the left edge (far = pinchStartGap
// + distance must not exceed x), so a fixed default like 200 overflowed a pinch
// centered at x≈200 on a narrow phone. Deriving it from x scales with the screen
// without needing the screen size (which mobilecli only knows in the natural
// orientation), and leaves the far edges to the device like the rest of pinch.
func autoPinchDistance(x int) int {
	d := (x - pinchStartGap) * 4 / 5
	if d < minPinchDistance {
		d = minPinchDistance
	}
	return d
}

// pinchActions lays two fingers on the horizontal line through (x, y), each
// pinchStartGap from the center, and moves them distance pixels away from it
// ("out", zooms in) or toward it ("in", zooms out) over duration milliseconds.
// Each finger is one complete, contiguous pointer sequence with its own Button,
// which is what devicekit.ConvertActions expects; the agents run both fingers
// at the same time.
//
// Like every other io command, this rejects negative coordinates and leaves
// the far edges to the device: agents may report the natural-orientation
// screen size, so checking against it would refuse valid pinches on a rotated
// screen.
func pinchActions(x, y int, direction string, distance, duration int) ([]devicekit.TapAction, error) {
	if distance <= 0 {
		distance = autoPinchDistance(x)
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

	if y < 0 {
		return nil, fmt.Errorf("pinch center y must be non-negative, got %d", y)
	}
	if x-far < 0 {
		return nil, fmt.Errorf("pinch centered at (%d,%d) with distance %d would put the left finger at x=%d, past the left edge", x, y, distance, x-far)
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

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
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
	elements, err := dumpElementsWithRefs(device, ref)
	if err != nil {
		return 0, 0, err
	}

	return centerOfRef(elements, ref)
}

func dumpElementsWithRefs(device devices.ControllableDevice, ref string) ([]types.ScreenElement, error) {
	elements, err := device.DumpSource(devices.DumpOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to dump UI to resolve ref %s: %w", ref, err)
	}

	types.AttachRefs(elements)
	return elements, nil
}

func centerOfRef(elements []types.ScreenElement, ref string) (int, int, error) {
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

// doubleTapActions taps (x, y) twice. A finger touches down only once per
// gesture, so the second tap is a second finger on the same spot that pauses
// until the first has lifted.
func doubleTapActions(x, y int) []devicekit.TapAction {
	return []devicekit.TapAction{
		{Type: "pointerMove", X: x, Y: y, Button: 0},
		{Type: "pointerDown", Button: 0},
		{Type: "pause", Duration: doubleTapHoldMs, Button: 0},
		{Type: "pointerUp", Button: 0},
		{Type: "pause", Duration: doubleTapHoldMs + doubleTapGapMs, Button: 1},
		{Type: "pointerMove", X: x, Y: y, Button: 1},
		{Type: "pointerDown", Button: 1},
		{Type: "pause", Duration: doubleTapHoldMs, Button: 1},
		{Type: "pointerUp", Button: 1},
	}
}

// DoubleTapCommand performs a double tap operation on the specified device
func DoubleTapCommand(req DoubleTapRequest) *CommandResponse {
	if req.Ref == "" && (req.X < 0 || req.Y < 0) {
		return NewErrorResponse(fmt.Errorf("x and y coordinates must be non-negative, got x=%d, y=%d", req.X, req.Y))
	}

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	x, y := req.X, req.Y
	if req.Ref != "" {
		x, y, err = resolveRefTapPoint(targetDevice, req.Ref)
		if err != nil {
			return NewErrorResponse(err)
		}
	}

	err = targetDevice.Gesture(doubleTapActions(x, y))
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to double tap on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Double tapped on device %s at (%d,%d)", targetDevice.ID(), x, y),
	})
}

// dragActions presses one finger at (x1, y1), keeps it still for holdMs, moves
// it to (x2, y2) over durationMs and lifts it there.
func dragActions(x1, y1, x2, y2, holdMs, durationMs int) []devicekit.TapAction {
	return []devicekit.TapAction{
		{Type: "pointerMove", X: x1, Y: y1},
		{Type: "pointerDown"},
		{Type: "pause", Duration: holdMs},
		{Type: "pointerMove", X: x2, Y: y2, Duration: durationMs},
		{Type: "pointerUp"},
	}
}

func validateDragPoint(name string, point DragPoint) error {
	if point.Ref == "" && (point.X < 0 || point.Y < 0) {
		return fmt.Errorf("'%s' coordinates must be non-negative, got x=%d, y=%d", name, point.X, point.Y)
	}
	return nil
}

// resolveDragPoints turns both ends of a drag into coordinates. Both refs are
// read from one dump, so they are numbered against the same screen.
func resolveDragPoints(device devices.ControllableDevice, from, to DragPoint) (x1, y1, x2, y2 int, err error) {
	x1, y1, x2, y2 = from.X, from.Y, to.X, to.Y
	if from.Ref == "" && to.Ref == "" {
		return x1, y1, x2, y2, nil
	}

	elements, err := dumpElementsWithRefs(device, strings.TrimSpace(from.Ref+" "+to.Ref))
	if err != nil {
		return 0, 0, 0, 0, err
	}

	if from.Ref != "" {
		x1, y1, err = centerOfRef(elements, from.Ref)
		if err != nil {
			return 0, 0, 0, 0, err
		}
	}
	if to.Ref != "" {
		x2, y2, err = centerOfRef(elements, to.Ref)
		if err != nil {
			return 0, 0, 0, 0, err
		}
	}
	return x1, y1, x2, y2, nil
}

// DragCommand performs a one-finger drag on the specified device
func DragCommand(req DragRequest) *CommandResponse {
	if err := validateDragPoint("from", req.From); err != nil {
		return NewErrorResponse(err)
	}
	if err := validateDragPoint("to", req.To); err != nil {
		return NewErrorResponse(err)
	}
	if req.HoldMs < 0 {
		return NewErrorResponse(fmt.Errorf("hold must not be negative, got %dms", req.HoldMs))
	}
	if req.DurationMs < 0 {
		return NewErrorResponse(fmt.Errorf("duration must not be negative, got %dms", req.DurationMs))
	}

	durationMs := req.DurationMs
	if durationMs == 0 {
		durationMs = DefaultDragDurationMs
	}

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	x1, y1, x2, y2, err := resolveDragPoints(targetDevice, req.From, req.To)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = targetDevice.Gesture(dragActions(x1, y1, x2, y2, req.HoldMs, durationMs))
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to drag on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Dragged on device %s from (%d,%d) to (%d,%d)", targetDevice.ID(), x1, y1, x2, y2),
	})
}

// LongPressCommand performs a long press operation on the specified device
func LongPressCommand(req LongPressRequest) *CommandResponse {
	if req.Ref == "" && (req.X < 0 || req.Y < 0) {
		return NewErrorResponse(fmt.Errorf("x and y coordinates must be non-negative, got x=%d, y=%d", req.X, req.Y))
	}

	// turned away here, not left to the device: the ios agent crashes when it is
	// asked to lift a finger before it touched down
	if req.Duration < 0 {
		return NewErrorResponse(fmt.Errorf("duration must not be negative, got %dms", req.Duration))
	}

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	x, y := req.X, req.Y
	if req.Ref != "" {
		x, y, err = resolveRefTapPoint(targetDevice, req.Ref)
		if err != nil {
			return NewErrorResponse(err)
		}
	}

	err = targetDevice.LongPress(x, y, req.Duration)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to long press on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Long pressed on device %s at (%d,%d) for %dms", targetDevice.ID(), x, y, req.Duration),
	})
}

// TextCommand sends text input to the specified device
func TextCommand(req TextRequest) *CommandResponse {
	if req.Text == "" {
		return NewErrorResponse(fmt.Errorf("text is required"))
	}

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
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
// normalizeButtonName canonicalizes a hardware button name so the command is
// case-insensitive, as the help documents ("Button names are case-insensitive").
// The per-platform button maps are keyed by upper-case names (HOME, VOLUME_UP),
// so upper-casing here lets "home" and "Home" resolve the same as "HOME".
func normalizeButtonName(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

func ButtonCommand(req ButtonRequest) *CommandResponse {
	button := normalizeButtonName(req.Button)
	if button == "" {
		return NewErrorResponse(fmt.Errorf("button name is required"))
	}

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = targetDevice.PressButton(button)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to press button on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Pressed button '%s' on device %s", button, targetDevice.ID()),
	})
}

// GestureCommand performs a gesture operation on the specified device
func GestureCommand(req GestureRequest) *CommandResponse {
	if len(req.Actions) == 0 {
		return NewErrorResponse(fmt.Errorf("actions array is required and cannot be empty"))
	}

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
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

	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
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
	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	err = targetDevice.Swipe(req.X1, req.Y1, req.X2, req.Y2, req.Duration)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to swipe on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Swiped on device %s from (%d,%d) to (%d,%d)", targetDevice.ID(), req.X1, req.Y1, req.X2, req.Y2),
	})
}
