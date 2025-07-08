package wda

import (
	"fmt"
)

type TapAction struct {
	Type     string `json:"type"`
	Duration int    `json:"duration,omitempty"`
	X        int    `json:"x,omitempty"`
	Y        int    `json:"y,omitempty"`
	Button   int    `json:"button,omitempty"`
}

type PointerParameters struct {
	PointerType string `json:"pointerType"`
}

type Pointer struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Parameters PointerParameters `json:"parameters"`
	Actions    []TapAction       `json:"actions"`
}

type ActionsRequest struct {
	Actions []Pointer `json:"actions"`
}

func Tap(x, y int) error {

	sessionId, err := CreateSession()
	if err != nil {
		return err
	}

	defer DeleteSession(sessionId)

	data := ActionsRequest{
		Actions: []Pointer{
			{
				Type: "pointer",
				ID:   "finger1",
				Parameters: PointerParameters{
					PointerType: "touch",
				},
				Actions: []TapAction{
					{Type: "pointerMove", Duration: 0, X: x, Y: y},
					{Type: "pointerDown", Button: 0},
					{Type: "pause", Duration: 100},
					{Type: "pointerUp", Button: 0},
				},
			},
		},
	}

	_, err = PostWebDriverAgentEndpoint(fmt.Sprintf("session/%s/actions", sessionId), data)
	if err != nil {
		return err
	}
	return nil
}
