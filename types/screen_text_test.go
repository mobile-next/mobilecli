package types

import "testing"

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func TestFormatTextIndentsTreeAndShowsRefsLabelsAndStates(t *testing.T) {
	elements := []ScreenElement{
		{
			Type: "Window",
			Children: []ScreenElement{
				{
					Type:       "Button",
					Label:      stringPtr("Add"),
					Identifier: stringPtr("add-button"),
					Enabled:    boolPtr(false),
				},
				{
					Type:        "TextField",
					Placeholder: stringPtr("First name"),
					Focused:     boolPtr(true),
				},
			},
		},
	}
	AttachRefs(elements)

	got := FormatText(elements)
	want := "@e1 [Window]\n" +
		"  @e2 [Button] \"Add\" #add-button (disabled)\n" +
		"  @e3 [TextField] \"First name\" (focused)\n"

	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatTextPrefersTextOverOtherLabels(t *testing.T) {
	elements := []ScreenElement{
		{Type: "StaticText", Text: stringPtr("Hello"), Label: stringPtr("ignored")},
	}
	AttachRefs(elements)

	want := "@e1 [StaticText] \"Hello\"\n"
	if got := FormatText(elements); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
