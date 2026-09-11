package devicekit

import (
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
