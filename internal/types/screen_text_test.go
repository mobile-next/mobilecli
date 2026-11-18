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

func TestFormatTextShowsValueAfterColonWhenItDiffersFromDescription(t *testing.T) {
	elements := []ScreenElement{
		{Type: "TextField", Label: stringPtr("Email"), Identifier: stringPtr("email"), Value: stringPtr("gil@example.com"), Focused: boolPtr(true)},
		{Type: "TextField", Label: stringPtr("Phone"), Value: stringPtr("")},
		{Type: "TextField", Value: stringPtr("only value")},
	}
	AttachRefs(elements)

	want := "@e1 [TextField] \"Email\" #email (focused): \"gil@example.com\"\n" +
		"@e2 [TextField] \"Phone\"\n" +
		"@e3 [TextField] \"only value\"\n"
	if got := FormatText(elements); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
