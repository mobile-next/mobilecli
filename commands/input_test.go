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

func TestAutoPinchDistanceKeepsInnerFingerOnScreen(t *testing.T) {
	// For a centered pinch, far = pinchStartGap + distance must not pass the
	// left edge (x - far >= 0). The old fixed default of 200 failed at x≈201.
	for _, x := range []int{201, 180, 300, 540, 640} {
		d := autoPinchDistance(x)
		far := pinchStartGap + d
		if x-far < 0 {
			t.Errorf("autoPinchDistance(%d)=%d -> far=%d puts the inner finger at x=%d (off-screen left)", x, d, far, x-far)
		}
		if d < minPinchDistance {
			t.Errorf("autoPinchDistance(%d)=%d is below the minimum %d", x, d, minPinchDistance)
		}
	}
}

func TestPinchActionsAutoDistanceSucceedsAtNarrowCenter(t *testing.T) {
	// A default pinch centered on a 402pt-wide screen (x=201) used to error with
	// "past the left edge"; with the auto distance it must build valid actions.
	for _, dir := range []string{PinchDirectionOut, PinchDirectionIn} {
		if _, err := pinchActions(201, 437, dir, 0, 0); err != nil {
			t.Errorf("pinchActions at a narrow center (%s) returned error: %v", dir, err)
		}
	}
}
