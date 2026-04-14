package wda

import (
	"encoding/json"
	"fmt"
)

type gestureAction struct {
	Type     string  `json:"type"`
	Duration float64 `json:"duration"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Button   int     `json:"button"`
}

// convertActions converts WDA-style TapActions to devicekit-ios gesture actions.
// WDA uses: pointerMove (position) -> pointerDown -> pointerMove (drag) -> pointerUp
// devicekit-ios uses: press (with position) -> move -> release
func convertActions(actions []TapAction) []gestureAction {
	var result []gestureAction
	pressed := false
	var pendingX, pendingY float64

	for i := range actions {
		a := &actions[i]
		switch a.Type {
		case "pointerMove":
			if !pressed {
				// pointerMove before pointerDown is just positioning
				pendingX, pendingY = float64(a.X), float64(a.Y)
			} else {
				// pointerMove after pointerDown is a drag
				result = append(result, gestureAction{
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
			if pendingX != 0 || pendingY != 0 {
				x, y = pendingX, pendingY
			}
			result = append(result, gestureAction{
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
			result = append(result, gestureAction{
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

func (c *WdaClient) Gesture(actions []TapAction) error {
	params := map[string]any{
		"actions": convertActions(actions),
	}

	_, err := c.CallRPC("device.io.gesture", params)
	return err
}

func (c *WdaClient) GestureFromJSON(jsonData []byte) error {
	var actions []TapAction
	if err := json.Unmarshal(jsonData, &actions); err != nil {
		return fmt.Errorf("failed to parse gesture actions: %v", err)
	}

	return c.Gesture(actions)
}
