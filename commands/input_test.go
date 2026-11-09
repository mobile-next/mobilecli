package commands

import (
	"strings"
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

	element := findElementByRef(elements, "@e2")
	if element == nil {
		t.Fatal("expected to find @e2")
	}
	if element.Type != "button" {
		t.Fatalf("expected button, got %s", element.Type)
	}

	if findElementByRef(elements, "@e99") != nil {
		t.Fatal("expected @e99 to be missing")
	}
}

func TestLongPressCommandRejectsANegativeDuration(t *testing.T) {
	// no such device: a negative duration must be turned away before any device or
	// agent is involved, because the ios agent crashes when it is handed one
	response := LongPressCommand(LongPressRequest{DeviceID: "no-such-device", X: 10, Y: 10, Duration: -100})

	if response.Status != "error" {
		t.Fatalf("expected an error response, got status %q", response.Status)
	}
	if !strings.Contains(response.Error, "duration must not be negative") {
		t.Errorf("expected the error to name the duration, got %q", response.Error)
	}
}

func TestNormalizeButtonNameIsCaseInsensitiveAndTrimmed(t *testing.T) {
	cases := map[string]string{
		"home":        "HOME",
		"Home":        "HOME",
		"HOME":        "HOME",
		"volume_up":   "VOLUME_UP",
		"  home  ":    "HOME",
		"VolUme_Down": "VOLUME_DOWN",
		"":            "",
		"   ":         "",
	}
	for in, want := range cases {
		if got := normalizeButtonName(in); got != want {
			t.Errorf("normalizeButtonName(%q) = %q, want %q", in, got, want)
		}
	}
}
