package devicekit

import (
	"encoding/json"
	"fmt"
)

// GestureAction is one step of one finger in the contract both device agents
// speak: devicekit-ios's device.io.gesture and the Android DeviceServer's.
// Duration is in seconds; Button is the finger index.
type GestureAction struct {
	Type     string  `json:"type"`
	Duration float64 `json:"duration"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Button   int     `json:"button"`
	// Start delays this finger's press by that many seconds from the start of
	// the gesture; without it every finger touches down at once. Only set on a
	// press.
	Start float64 `json:"start,omitempty"`
}

// ConvertActions converts W3C pointer actions (TapActions) to the agents' gesture actions.
// W3C pointer actions use: pointerMove (position) -> pointerDown -> pointerMove (drag) -> pointerUp
// The agents use: press (with position) -> move -> release
//
// The conversion is stateful per sequence, not per finger: a multi-finger
// gesture must list each finger's actions contiguously (all of Button 0, then
// all of Button 1), never interleaved.
//
// A pause before a finger's pointerDown delays that finger's touch-down (the
// W3C meaning), which is how a gesture taps more than once: a double tap is a
// second finger on the same spot that pauses until the first has lifted.
func ConvertActions(actions []TapAction) []GestureAction {
	var result []GestureAction
	pressed := false
	hasPendingPos := false
	var pendingX, pendingY float64
	var pendingStart float64
	touched := map[int]bool{}

	for i := range actions {
		a := &actions[i]
		switch a.Type {
		case "pointerMove":
			if !pressed {
				// pointerMove before pointerDown is just positioning
				pendingX, pendingY = float64(a.X), float64(a.Y)
				hasPendingPos = true
			} else {
				// pointerMove after pointerDown is a drag
				result = append(result, GestureAction{
					Type:     "move",
					Duration: float64(a.Duration) / 1000.0,
					X:        float64(a.X),
					Y:        float64(a.Y),
					Button:   a.Button,
				})
			}
		case "pointerDown":
			pressed = true
			touched[a.Button] = true
			x, y := float64(a.X), float64(a.Y)
			if hasPendingPos {
				x, y = pendingX, pendingY
				hasPendingPos = false
			}
			result = append(result, GestureAction{
				Type:     "press",
				Duration: float64(a.Duration) / 1000.0,
				X:        x,
				Y:        y,
				Button:   a.Button,
				Start:    pendingStart,
			})
			pendingStart = 0
		case "pointerUp":
			pressed = false
			x, y := float64(a.X), float64(a.Y)
			if len(result) > 0 {
				last := result[len(result)-1]
				x, y = last.X, last.Y
			}
			result = append(result, GestureAction{
				Type:     "release",
				Duration: float64(a.Duration) / 1000.0,
				X:        x,
				Y:        y,
				Button:   a.Button,
			})
		case "pause":
			// before this finger's first touch, a pause delays it; otherwise it
			// extends the duration of the previous action
			if !pressed && !touched[a.Button] {
				pendingStart += float64(a.Duration) / 1000.0
			} else if len(result) > 0 {
				result[len(result)-1].Duration += float64(a.Duration) / 1000.0
			}
		}
	}
	return result
}

func (c *DeviceKitClient) Gesture(actions []TapAction) error {
	params := map[string]any{
		"actions": ConvertActions(actions),
	}

	_, err := c.CallRPC("device.io.gesture", params)
	return err
}

func (c *DeviceKitClient) GestureFromJSON(jsonData []byte) error {
	var actions []TapAction
	if err := json.Unmarshal(jsonData, &actions); err != nil {
		return fmt.Errorf("failed to parse gesture actions: %v", err)
	}

	return c.Gesture(actions)
}
