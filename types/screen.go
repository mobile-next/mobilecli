package types

import "fmt"

type ScreenElementRect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ScreenElement struct {
	Ref         string            `json:"ref,omitempty"`
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

// AttachRefs assigns each element a ref ("e1".."eN") in depth-first pre-order,
// so a ref is the element's position in the tree as printed. Refs are only
// valid against the dump that produced them.
func AttachRefs(elements []ScreenElement) {
	counter := 0
	attachRefs(elements, &counter)
}

func attachRefs(elements []ScreenElement, counter *int) {
	for i := range elements {
		*counter++
		elements[i].Ref = fmt.Sprintf("e%d", *counter)
		attachRefs(elements[i].Children, counter)
	}
}
