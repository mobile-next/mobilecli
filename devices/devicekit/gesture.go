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
}

// ConvertActions converts WDA-style TapActions to the agents' gesture actions.
// WDA uses: pointerMove (position) -> pointerDown -> pointerMove (drag) -> pointerUp
// The agents use: press (with position) -> move -> release
//
// The conversion is stateful per sequence, not per finger: a multi-finger
// gesture must list each finger's actions contiguously (all of Button 0, then
// all of Button 1), never interleaved.
func ConvertActions(actions []TapAction) []GestureAction {
	var result []GestureAction
	pressed := false
	hasPendingPos := false
	var pendingX, pendingY float64

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
			})
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
			// pause extends the duration of the previous action
			if len(result) > 0 {
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
