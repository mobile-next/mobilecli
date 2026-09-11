package devices

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestKeysParamsCarryKeycodeAndModifiersPerKey(t *testing.T) {
	payload, err := json.Marshal(keysParams{Keys: []keyParams{
		{Keycode: "KEYCODE_A", Modifiers: []string{"KEYCODE_CTRL_LEFT"}},
		{Keycode: "KEYCODE_ENTER"},
	}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"keys":[{"keycode":"KEYCODE_A","modifiers":["KEYCODE_CTRL_LEFT"]},{"keycode":"KEYCODE_ENTER"}]}`, string(payload))
}

func TestInputParamsUseTheOpenRpcFieldNames(t *testing.T) {
	assert.ElementsMatch(t, []string{"x", "y"}, wireKeys(t, tapParams{X: 1, Y: 2}))
	assert.ElementsMatch(t, []string{"x", "y", "duration"}, wireKeys(t, longPressParams{X: 1, Y: 2, Duration: 500}))
	assert.ElementsMatch(t, []string{"x1", "y1", "x2", "y2", "duration"}, wireKeys(t, swipeParams{X1: 1, Y1: 2, X2: 3, Y2: 4, Duration: 300}))
	assert.ElementsMatch(t, []string{"button"}, wireKeys(t, buttonParams{Button: "KEYCODE_HOME"}))
	assert.ElementsMatch(t, []string{"text"}, wireKeys(t, textParams{Text: "hi"}))
}

// freePort returns a port with nothing listening on it.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func TestAgentRequestReportsAnAgentItCannotReachAsUnreachable(t *testing.T) {
	_, err := agentRequest(freePort(t), "device.version", nil)

	assert.ErrorIs(t, err, errAgentUnreachable)
}

func TestAgentRequestReportsAnErrorTheAgentItselfSentAsSomethingElse(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","error":{"code":-32601,"message":"Method not found"}}`))
	}))
	defer agent.Close()
	port := agent.Listener.Addr().(*net.TCPAddr).Port

	_, err := agentRequest(port, "device.nonsense", nil)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, errAgentUnreachable)
}

func TestAgentRequestReportsATimeoutSeparatelyFromAnUnreachableAgent(t *testing.T) {
	slowAgent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer slowAgent.Close()
	port := slowAgent.Listener.Addr().(*net.TCPAddr).Port

	_, err := agentRequestWithTimeout(port, "device.dump.ui", nil, 10*time.Millisecond)

	assert.ErrorIs(t, err, errAgentTimedOut)
	assert.NotErrorIs(t, err, errAgentUnreachable, "a timed-out call must not be resent")
}

func TestPressingNoKeysDoesNothingRatherThanAskingTheServerToPressNothing(t *testing.T) {
	device := &AndroidDevice{id: "no-such-device"}

	assert.NoError(t, device.PressKeys(nil))
	assert.NoError(t, device.PressKeys([]KeyCombo{}))
}
