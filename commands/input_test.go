package commands

import (
	"testing"

	"github.com/mobile-next/mobilecli/types"
)

func TestFindElementByRefSearchesNestedChildren(t *testing.T) {
	elements := []types.ScreenElement{
		{
			Type: "window",
			Children: []types.ScreenElement{
				{Type: "button", Rect: types.ScreenElementRect{X: 10, Y: 20, Width: 100, Height: 40}},
			},
		},
	}
	types.AttachRefs(elements)

	element := findElementByRef(elements, "e2")
	if element == nil {
		t.Fatal("expected to find e2")
	}
	if element.Type != "button" {
		t.Fatalf("expected button, got %s", element.Type)
	}

	if findElementByRef(elements, "e99") != nil {
		t.Fatal("expected e99 to be missing")
	}
}
