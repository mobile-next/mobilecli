package types

type ScreenElementRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ScreenElement struct {
	Type        string            `json:"type"`
	Label       *string           `json:"label,omitempty"`
	Text        *string           `json:"text,omitempty"`
	Name        *string           `json:"name,omitempty"`
	Value       *string           `json:"value,omitempty"`
	Placeholder *string           `json:"placeholder,omitempty"`
	Identifier  *string           `json:"identifier,omitempty"`
	Rect        ScreenElementRect `json:"rect"`
	Focused     *bool             `json:"focused,omitempty"`  // android tv, and ios when hasFocus is reported
	Enabled     *bool             `json:"enabled,omitempty"`  // only set when false
	Checked     *bool             `json:"checked,omitempty"`  // only set when true
	Selected    *bool             `json:"selected,omitempty"` // only set when true
	Children    []ScreenElement   `json:"children,omitempty"`
}
