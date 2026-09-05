package types

import "testing"

func TestAttachRefsNumbersDepthFirstPreOrder(t *testing.T) {
	elements := []ScreenElement{
		{
			Type: "window",
			Children: []ScreenElement{
				{Type: "button"},
				{Type: "list", Children: []ScreenElement{{Type: "cell"}}},
			},
		},
		{Type: "toolbar"},
	}

	AttachRefs(elements)

	got := []string{
		elements[0].Ref,
		elements[0].Children[0].Ref,
		elements[0].Children[1].Ref,
		elements[0].Children[1].Children[0].Ref,
		elements[1].Ref,
	}
	want := []string{"e1", "e2", "e3", "e4", "e5"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ref %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
