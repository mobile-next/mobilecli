package wda

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