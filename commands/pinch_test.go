package commands

import (
	"testing"

	"github.com/mobile-next/mobilecli/devices/devicekit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pointerMove = "pointerMove"

// fingerPath returns the x coordinates a finger touches down at and lifts from.
func fingerPath(t *testing.T, actions []devicekit.TapAction, finger int) (startX, endX int) {
	t.Helper()
	var moves []devicekit.TapAction
	for _, a := range actions {
		if a.Button == finger && a.Type == pointerMove {
			moves = append(moves, a)
		}
	}
	require.Len(t, moves, 2, "finger %d should position once and drag once", finger)
	return moves[0].X, moves[1].X
}

func TestPinchOutSpreadsFingersAwayFromTheCenter(t *testing.T) {
	actions, err := pinchActions(640, 1428, PinchDirectionOut, 200, 300)
	require.NoError(t, err)

	leftStart, leftEnd := fingerPath(t, actions, 0)
	rightStart, rightEnd := fingerPath(t, actions, 1)
	assert.Equal(t, 610, leftStart)
	assert.Equal(t, 410, leftEnd)
	assert.Equal(t, 670, rightStart)
	assert.Equal(t, 870, rightEnd)

	for _, a := range actions {
		if a.Type == pointerMove {
			assert.Equal(t, 1428, a.Y, "fingers stay on the horizontal line through the center")
		}
	}
}

func TestPinchInBringsFingersTowardTheCenter(t *testing.T) {
	actions, err := pinchActions(640, 1428, PinchDirectionIn, 200, 300)
	require.NoError(t, err)

	leftStart, leftEnd := fingerPath(t, actions, 0)
	rightStart, rightEnd := fingerPath(t, actions, 1)
	assert.Equal(t, 410, leftStart)
	assert.Equal(t, 610, leftEnd)
	assert.Equal(t, 870, rightStart)
	assert.Equal(t, 670, rightEnd)
}

func TestPinchAppliesDefaultDistanceAndDuration(t *testing.T) {
	actions, err := pinchActions(640, 1428, PinchDirectionOut, 0, 0)
	require.NoError(t, err)

	leftStart, leftEnd := fingerPath(t, actions, 0)
	assert.Equal(t, defaultPinchDistance, leftStart-leftEnd)

	for _, a := range actions {
		if a.Type == pointerMove && a.Duration != 0 {
			assert.Equal(t, defaultPinchDurationMs, a.Duration)
		}
	}
}

func TestPinchListsEachFingerContiguously(t *testing.T) {
	actions, err := pinchActions(640, 1428, PinchDirectionOut, 200, 300)
	require.NoError(t, err)
	require.Len(t, actions, 10)

	// devicekit.ConvertActions is stateful per sequence, so finger 1 must not
	// start until finger 0 has lifted
	for i, a := range actions[:5] {
		assert.Equal(t, 0, a.Button, "action %d belongs to finger 0", i)
	}
	for i, a := range actions[5:] {
		assert.Equal(t, 1, a.Button, "action %d belongs to finger 1", i+5)
	}
	assert.Equal(t, "pointerUp", actions[4].Type)
	assert.Equal(t, pointerMove, actions[5].Type)
	assert.Equal(t, "pointerDown", actions[6].Type)

	converted := devicekit.ConvertActions(actions)
	require.Len(t, converted, 6)
	assert.Equal(t, []string{"press", "move", "release", "press", "move", "release"}, actionTypes(converted))
}

func actionTypes(actions []devicekit.GestureAction) []string {
	types := make([]string, len(actions))
	for i, a := range actions {
		types[i] = a.Type
	}
	return types
}

func TestPinchRejectsUnknownDirection(t *testing.T) {
	_, err := pinchActions(640, 1428, "sideways", 200, 300)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "direction")
}

func TestPinchRejectsAFingerThatWouldLeaveTheScreen(t *testing.T) {
	_, err := pinchActions(100, 1428, PinchDirectionIn, 200, 300)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not fit on screen")
}

func TestPinchCommandNamesTheDirectionBeforeLookingForADevice(t *testing.T) {
	resp := PinchCommand(PinchRequest{DeviceID: "__nope__", Direction: "sideways"})
	require.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "direction")
	assert.NotContains(t, resp.Error, "__nope__")
}
