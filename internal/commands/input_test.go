package commands

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mobile-next/mobilecli/internal/devices/devicekit"
	"github.com/mobile-next/mobilecli/internal/types"
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

func TestDoubleTapActionsTapTwiceWithASecondFingerThatWaitsForTheFirstToLift(t *testing.T) {
	converted := devicekit.ConvertActions(doubleTapActions(100, 200))

	want := []devicekit.GestureAction{
		{Type: "press", X: 100, Y: 200, Duration: 0.05, Button: 0},
		{Type: "release", X: 100, Y: 200, Button: 0},
		{Type: "press", X: 100, Y: 200, Duration: 0.05, Button: 1, Start: 0.2},
		{Type: "release", X: 100, Y: 200, Button: 1},
	}
	if !reflect.DeepEqual(converted, want) {
		t.Errorf("double tap converted to %+v, want %+v", converted, want)
	}
}

func TestDoubleTapCommandRejectsNegativeCoordinates(t *testing.T) {
	// no such device: bad coordinates are turned away before any device is involved
	response := DoubleTapCommand(DoubleTapRequest{DeviceID: "no-such-device", X: -1, Y: 10})

	if response.Status != "error" {
		t.Fatalf("expected an error response, got status %q", response.Status)
	}
	if !strings.Contains(response.Error, "must be non-negative") {
		t.Errorf("expected the error to name the coordinates, got %q", response.Error)
	}
}

func TestDragActionsPressHoldTravelAndLiftWithOneFinger(t *testing.T) {
	converted := devicekit.ConvertActions(dragActions(68, 729, 322, 729, 500, 800))

	want := []devicekit.GestureAction{
		{Type: "press", X: 68, Y: 729, Duration: 0.5},
		{Type: "move", X: 322, Y: 729, Duration: 0.8},
		{Type: "release", X: 322, Y: 729},
	}
	if !reflect.DeepEqual(converted, want) {
		t.Errorf("drag converted to %+v, want %+v", converted, want)
	}
}

func TestDragActionsWithoutAHoldStartMovingRightAway(t *testing.T) {
	converted := devicekit.ConvertActions(dragActions(10, 10, 50, 50, 0, 800))

	if converted[0].Duration != 0 {
		t.Errorf("expected the press not to hold, got %vs", converted[0].Duration)
	}
}

func TestDragCommandRejectsBadInputBeforeLookingForADevice(t *testing.T) {
	from, to := DragPoint{X: 10, Y: 10}, DragPoint{X: 50, Y: 50}
	cases := map[string]struct {
		request DragRequest
		mention string
	}{
		"negative start":    {DragRequest{From: DragPoint{X: -1, Y: 10}, To: to}, "from"},
		"negative end":      {DragRequest{From: from, To: DragPoint{X: 50, Y: -5}}, "to"},
		"negative hold":     {DragRequest{From: from, To: to, HoldMs: -1}, "hold"},
		"negative duration": {DragRequest{From: from, To: to, DurationMs: -1}, "duration"},
	}

	for name, c := range cases {
		request := c.request
		request.DeviceID = "no-such-device"

		response := DragCommand(request)

		if response.Status != "error" {
			t.Fatalf("%s: expected an error response, got status %q", name, response.Status)
		}
		if !strings.Contains(response.Error, c.mention) || strings.Contains(response.Error, "no-such-device") {
			t.Errorf("%s: expected the error to name %q, got %q", name, c.mention, response.Error)
		}
	}
}
