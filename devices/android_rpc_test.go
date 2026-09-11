package devices

import (
	"encoding/json"
	"testing"

	"github.com/mobile-next/mobilecli/devices/devicekit"
	"github.com/mobile-next/mobilecli/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wireKeys returns the top-level JSON keys a value marshals to.
func wireKeys(t *testing.T, v any) []string {
	t.Helper()
	payload, err := json.Marshal(v)
	require.NoError(t, err)
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &object))
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

func TestAgentRequestLeavesParamsOutWhenThereAreNone(t *testing.T) {
	payload, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", ID: "1", Method: "device.version"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":"1","method":"device.version"}`, string(payload))
}

func TestGestureParamsCarryEachActionWithItsFinger(t *testing.T) {
	actions := devicekit.ConvertActions([]devicekit.TapAction{
		{Type: "pointerMove", X: 610, Y: 1428, Button: 0},
		{Type: "pointerDown", Button: 0},
		{Type: "pointerUp", Button: 0},
		{Type: "pointerMove", X: 670, Y: 1428, Button: 1},
		{Type: "pointerDown", Button: 1},
		{Type: "pointerUp", Button: 1},
	})

	payload, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", ID: "1", Method: "device.io.gesture", Params: gestureParams{Actions: actions}})
	require.NoError(t, err)

	var wire struct {
		Params struct {
			Actions []map[string]any `json:"actions"`
		} `json:"params"`
	}
	require.NoError(t, json.Unmarshal(payload, &wire))
	require.Len(t, wire.Params.Actions, 4)

	assert.Equal(t, "press", wire.Params.Actions[0]["type"])
	assert.Equal(t, float64(0), wire.Params.Actions[0]["button"])
	assert.Equal(t, "press", wire.Params.Actions[2]["type"])
	assert.Equal(t, float64(1), wire.Params.Actions[2]["button"])
	assert.Equal(t, float64(670), wire.Params.Actions[2]["x"])
}

func TestScreenshotParamsUseTheKeysDeviceServerReads(t *testing.T) {
	plain := screenshotParams{Format: "png", Quality: 90, Scale: 1, MaxSize: 0}
	assert.ElementsMatch(t, []string{"format", "quality", "scale", "maxSize"}, wireKeys(t, plain))

	clipped := plain
	clipped.Clip = &types.ScreenElementRect{X: 1, Y: 2, Width: 3, Height: 4}
	clipped.ScreenWidth = 412
	assert.ElementsMatch(t, []string{"format", "quality", "scale", "maxSize", "clip", "screenWidth"}, wireKeys(t, clipped))
	assert.ElementsMatch(t, []string{"x", "y", "width", "height"}, wireKeys(t, clipped.Clip))
}
