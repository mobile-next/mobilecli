package devicekit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertActionsKeepsTwoContiguousFingersApart(t *testing.T) {
	actions := []TapAction{
		{Type: "pointerMove", X: 100, Y: 500, Button: 0},
		{Type: "pointerDown", Button: 0},
		{Type: "pause", Duration: 50, Button: 0},
		{Type: "pointerMove", X: 10, Y: 500, Duration: 300, Button: 0},
		{Type: "pointerUp", Button: 0},
		{Type: "pointerMove", X: 200, Y: 500, Button: 1},
		{Type: "pointerDown", Button: 1},
		{Type: "pointerMove", X: 290, Y: 500, Duration: 300, Button: 1},
		{Type: "pointerUp", Button: 1},
	}

	converted := ConvertActions(actions)
	require.Len(t, converted, 6)

	assert.Equal(t, GestureAction{Type: "press", X: 100, Y: 500, Duration: 0.05, Button: 0}, converted[0])
	assert.Equal(t, GestureAction{Type: "move", X: 10, Y: 500, Duration: 0.3, Button: 0}, converted[1])
	assert.Equal(t, GestureAction{Type: "release", X: 10, Y: 500, Button: 0}, converted[2])

	// the second finger starts a fresh press at its own position, not the
	// first finger's last point
	assert.Equal(t, GestureAction{Type: "press", X: 200, Y: 500, Button: 1}, converted[3])
	assert.Equal(t, GestureAction{Type: "move", X: 290, Y: 500, Duration: 0.3, Button: 1}, converted[4])
	assert.Equal(t, GestureAction{Type: "release", X: 290, Y: 500, Button: 1}, converted[5])
}

func TestAPauseBeforePointerDownDelaysThatFinger(t *testing.T) {
	// a double tap: the second finger waits until the first has lifted
	actions := []TapAction{
		{Type: "pointerMove", X: 100, Y: 200, Button: 0},
		{Type: "pointerDown", Button: 0},
		{Type: "pause", Duration: 50, Button: 0},
		{Type: "pointerUp", Button: 0},
		{Type: "pause", Duration: 150, Button: 1},
		{Type: "pause", Duration: 50, Button: 1},
		{Type: "pointerMove", X: 100, Y: 200, Button: 1},
		{Type: "pointerDown", Button: 1},
		{Type: "pause", Duration: 50, Button: 1},
		{Type: "pointerUp", Button: 1},
	}

	converted := ConvertActions(actions)
	require.Len(t, converted, 4)

	assert.Equal(t, GestureAction{Type: "press", X: 100, Y: 200, Duration: 0.05, Button: 0}, converted[0])
	assert.Equal(t, GestureAction{Type: "release", X: 100, Y: 200, Button: 0}, converted[1])
	assert.Equal(t, GestureAction{Type: "press", X: 100, Y: 200, Duration: 0.05, Button: 1, Start: 0.2}, converted[2])
	assert.Equal(t, GestureAction{Type: "release", X: 100, Y: 200, Button: 1}, converted[3])
}

func TestAPauseAfterAFingerLiftsStillExtendsItsRelease(t *testing.T) {
	actions := []TapAction{
		{Type: "pointerMove", X: 10, Y: 20},
		{Type: "pointerDown"},
		{Type: "pointerUp"},
		{Type: "pause", Duration: 100},
	}

	converted := ConvertActions(actions)
	require.Len(t, converted, 2)
	assert.InDelta(t, 0.1, converted[1].Duration, 1e-9)
	assert.Zero(t, converted[0].Start)
}

func TestStartIsLeftOutOfTheJSONWhenUnset(t *testing.T) {
	data, err := json.Marshal(GestureAction{Type: "press", X: 1, Y: 2})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "start")
}
