package devices

import "testing"

func TestFlattenElementsIncludesNestedChildren(t *testing.T) {
	name := "BroadcastUploadExtension"
	tree := []ScreenElement{
		{
			Type: "Window",
			Children: []ScreenElement{
				{Type: "Button", Name: &name},
			},
		},
	}

	flat := flattenElements(tree)

	if len(flat) != 2 {
		t.Fatalf("expected 2 elements after flatten, got %d", len(flat))
	}
	if flat[1].Name == nil || *flat[1].Name != name {
		t.Errorf("expected nested button to be flattened, got %+v", flat[1])
	}
}
