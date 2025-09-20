package types

type ScreenElementRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ScreenElement struct {
	Type       string             `json:"type"`
	Label      *string            `json:"label,omitempty"`
	Text       *string            `json:"text,omitempty"`
	Name       *string            `json:"name,omitempty"`
	Value      *string            `json:"value,omitempty"`
	Identifier *string            `json:"identifier,omitempty"`
	Rect       ScreenElementRect  `json:"rect"`
	Focused    *bool              `json:"focused,omitempty"` // currently only on android tv
}