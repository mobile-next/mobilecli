package devicekit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/devices/wda"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type mockHandler func(method string, params json.RawMessage) (interface{}, error)

func newMockServer(handler mockHandler) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		for {
			var req struct {
				JSONRPC string          `json:"jsonrpc"`
				Method  string          `json:"method"`
				Params  json.RawMessage `json:"params"`
				ID      int64           `json:"id"`
			}
			if err := conn.ReadJSON(&req); err != nil {
				return
			}

			result, err := handler(req.Method, req.Params)
			if err != nil {
				resp := map[string]interface{}{
					"jsonrpc": "2.0",
					"error":   map[string]interface{}{"code": -32603, "message": err.Error()},
					"id":      req.ID,
				}
				_ = conn.WriteJSON(resp)
				continue
			}

			resultBytes, _ := json.Marshal(result)
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"result":  json.RawMessage(resultBytes),
				"id":      req.ID,
			}
			_ = conn.WriteJSON(resp)
		}
	})

	return httptest.NewServer(mux)
}

func newTestClient(server *httptest.Server) *Client {
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	return NewClient(u.Hostname(), port)
}

// --- Client tests ---

func TestNewClient_URLParsing(t *testing.T) {
	c := NewClient("localhost", 12004)
	assert.Equal(t, "http://localhost:12004", c.httpURL)
	assert.Equal(t, "ws://localhost:12004", c.wsURL)

	c = NewClient("192.168.1.1", 8100)
	assert.Equal(t, "http://192.168.1.1:8100", c.httpURL)
	assert.Equal(t, "ws://192.168.1.1:8100", c.wsURL)
}

func TestHealthCheck_Success(t *testing.T) {
	server := newMockServer(nil)
	defer server.Close()

	client := newTestClient(server)
	err := client.HealthCheck()
	assert.NoError(t, err)
}

func TestHealthCheck_Failure(t *testing.T) {
	client := NewClient("localhost", 1) // nothing listening
	err := client.HealthCheck()
	assert.Error(t, err)
}

func TestWaitForReady_Success(t *testing.T) {
	server := newMockServer(nil)
	defer server.Close()

	client := newTestClient(server)
	err := client.WaitForReady(2 * time.Second)
	assert.NoError(t, err)
}

func TestWaitForReady_Timeout(t *testing.T) {
	client := NewClient("localhost", 1)
	err := client.WaitForReady(1 * time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestClose(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	_, err := client.call("test", nil)
	require.NoError(t, err)

	client.Close()
	// wait for readLoop to exit after close
	time.Sleep(50 * time.Millisecond)

	// after close, next call should reconnect
	_, err = client.call("test", nil)
	assert.NoError(t, err)
}

// --- JSON-RPC tests ---

func TestCall_Success(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		return map[string]string{"status": "ok"}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	result, err := client.call("test.method", map[string]string{"key": "value"})
	require.NoError(t, err)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(result, &resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestCall_RPCError(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		return nil, assert.AnError
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	_, err := client.call("test.method", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JSON-RPC error")
}

func TestCall_MultipleSequential(t *testing.T) {
	var mu sync.Mutex
	calls := []string{}

	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		mu.Lock()
		calls = append(calls, method)
		mu.Unlock()
		return map[string]string{"method": method}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	for _, m := range []string{"device.io.tap", "device.screenshot", "device.dump.ui"} {
		result, err := client.call(m, map[string]string{"deviceId": ""})
		require.NoError(t, err)

		var resp map[string]string
		require.NoError(t, json.Unmarshal(result, &resp))
		assert.Equal(t, m, resp["method"])
	}

	mu.Lock()
	assert.Equal(t, []string{"device.io.tap", "device.screenshot", "device.dump.ui"}, calls)
	mu.Unlock()
}

func TestCall_Timeout(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		time.Sleep(3 * time.Second) // longer than timeout
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	_, err := client.callWithTimeout("slow.method", nil, 500*time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestCall_ConnectionRefused(t *testing.T) {
	client := NewClient("localhost", 1)
	_, err := client.call("test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")
}

func TestCall_ReconnectAfterDisconnect(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		return map[string]bool{"ok": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)

	_, err := client.call("test", nil)
	require.NoError(t, err)

	// explicitly close and wait for readLoop to detect it
	client.Close()
	time.Sleep(50 * time.Millisecond)

	// next call should reconnect automatically
	result, err := client.call("test", nil)
	require.NoError(t, err)

	var resp map[string]bool
	require.NoError(t, json.Unmarshal(result, &resp))
	assert.True(t, resp["ok"])
}

// --- Tap tests ---

func TestTap(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.io.tap", method)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(params, &p))
		assert.Equal(t, float64(100), p["x"])
		assert.Equal(t, float64(200), p["y"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.Tap(100, 200)
	assert.NoError(t, err)
}

// --- LongPress tests ---

func TestLongPress(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.io.longpress", method)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(params, &p))
		assert.Equal(t, float64(100), p["x"])
		assert.Equal(t, float64(200), p["y"])
		assert.Equal(t, 1.5, p["duration"]) // 1500ms -> 1.5s
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.LongPress(100, 200, 1500)
	assert.NoError(t, err)
}

// --- Swipe tests ---

func TestSwipe(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.io.swipe", method)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(params, &p))
		assert.Equal(t, float64(100), p["fromX"])
		assert.Equal(t, float64(200), p["fromY"])
		assert.Equal(t, float64(300), p["toX"])
		assert.Equal(t, float64(400), p["toY"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.Swipe(100, 200, 300, 400)
	assert.NoError(t, err)
}

// --- SendKeys tests ---

func TestSendKeys(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.io.text", method)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(params, &p))
		assert.Equal(t, "hello world", p["text"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.SendKeys("hello world")
	assert.NoError(t, err)
}

// --- PressButton tests ---

func TestPressButton(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantMethod string
		wantButton string
	}{
		{"home", "HOME", "device.io.button", "home"},
		{"volume up", "VOLUME_UP", "device.io.button", "volumeUp"},
		{"volume down", "VOLUME_DOWN", "device.io.button", "volumeDown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
				assert.Equal(t, tt.wantMethod, method)
				var p map[string]interface{}
				require.NoError(t, json.Unmarshal(params, &p))
				assert.Equal(t, tt.wantButton, p["button"])
				return map[string]bool{"success": true}, nil
			})
			defer server.Close()

			client := newTestClient(server)
			defer client.Close()

			err := client.PressButton(tt.key)
			assert.NoError(t, err)
		})
	}
}

func TestPressButton_Enter(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.io.text", method)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(params, &p))
		assert.Equal(t, "\n", p["text"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.PressButton("ENTER")
	assert.NoError(t, err)
}

func TestPressButton_Unsupported(t *testing.T) {
	server := newMockServer(nil)
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.PressButton("POWER")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported button")
}

// --- OpenURL tests ---

func TestOpenURL(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.url", method)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(params, &p))
		assert.Equal(t, "https://example.com", p["url"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.OpenURL("https://example.com")
	assert.NoError(t, err)
}

// --- Gesture conversion tests ---

func TestConvertGestureActions_TapSequence(t *testing.T) {
	actions := []wda.TapAction{
		{Type: "pointerMove", Duration: 0, X: 200, Y: 400},
		{Type: "pointerDown", Button: 0},
		{Type: "pause", Duration: 100},
		{Type: "pointerUp", Button: 0},
	}

	result := convertGestureActions(actions)

	require.Len(t, result, 3)

	assert.Equal(t, "move", result[0].Type)
	assert.Equal(t, float64(200), result[0].X)
	assert.Equal(t, float64(400), result[0].Y)

	assert.Equal(t, "press", result[1].Type)
	assert.Equal(t, float64(200), result[1].X)
	assert.Equal(t, float64(400), result[1].Y)

	assert.Equal(t, "release", result[2].Type)
	assert.InDelta(t, 0.1, result[2].Duration, 0.001) // 100ms -> 0.1s
}

func TestConvertGestureActions_SwipeSequence(t *testing.T) {
	actions := []wda.TapAction{
		{Type: "pointerMove", Duration: 0, X: 200, Y: 600},
		{Type: "pointerDown", Button: 0},
		{Type: "pointerMove", Duration: 1000, X: 200, Y: 200},
		{Type: "pointerUp", Button: 0},
	}

	result := convertGestureActions(actions)

	require.Len(t, result, 4)

	assert.Equal(t, "move", result[0].Type)
	assert.Equal(t, float64(200), result[0].X)
	assert.Equal(t, float64(600), result[0].Y)
	assert.Equal(t, 0.0, result[0].Duration)

	assert.Equal(t, "press", result[1].Type)
	assert.Equal(t, float64(200), result[1].X)
	assert.Equal(t, float64(200), result[1].Y)

	assert.Equal(t, "move", result[2].Type)
	assert.Equal(t, float64(200), result[2].X)
	assert.Equal(t, float64(200), result[2].Y)
	assert.Equal(t, 1.0, result[2].Duration) // 1000ms -> 1.0s

	assert.Equal(t, "release", result[3].Type)
}

func TestConvertGestureActions_LongPressSequence(t *testing.T) {
	actions := []wda.TapAction{
		{Type: "pointerMove", Duration: 0, X: 150, Y: 300},
		{Type: "pointerDown", Button: 0},
		{Type: "pause", Duration: 2000},
		{Type: "pointerUp", Button: 0},
	}

	result := convertGestureActions(actions)

	require.Len(t, result, 3)

	assert.Equal(t, "press", result[1].Type)
	assert.Equal(t, float64(150), result[1].X)
	assert.Equal(t, float64(300), result[1].Y)

	assert.Equal(t, "release", result[2].Type)
	assert.InDelta(t, 2.0, result[2].Duration, 0.001) // pause absorbed
}

func TestConvertGestureActions_Passthrough(t *testing.T) {
	actions := []wda.TapAction{
		{Type: "press", X: 100, Y: 200, Duration: 0, Button: 0},
		{Type: "move", X: 100, Y: 100, Duration: 300, Button: 0},
		{Type: "release", X: 100, Y: 100, Duration: 0, Button: 0},
	}

	result := convertGestureActions(actions)

	require.Len(t, result, 3)
	assert.Equal(t, "press", result[0].Type)
	assert.Equal(t, float64(100), result[0].X)
	assert.Equal(t, 0.3, result[1].Duration) // 300ms -> 0.3s
	assert.Equal(t, "release", result[2].Type)
}

func TestConvertGestureActions_Empty(t *testing.T) {
	result := convertGestureActions(nil)
	assert.Empty(t, result)
}

// --- Gesture RPC test ---

func TestGesture(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.io.gesture", method)
		var p struct {
			Actions []deviceKitAction `json:"actions"`
		}
		require.NoError(t, json.Unmarshal(params, &p))
		require.NotEmpty(t, p.Actions)
		assert.Equal(t, "move", p.Actions[0].Type)
		assert.Equal(t, "press", p.Actions[1].Type)
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.Gesture([]wda.TapAction{
		{Type: "pointerMove", Duration: 0, X: 200, Y: 400},
		{Type: "pointerDown", Button: 0},
		{Type: "pause", Duration: 100},
		{Type: "pointerUp", Button: 0},
	})
	assert.NoError(t, err)
}

// --- Screenshot tests ---

func TestTakeScreenshot(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.screenshot", method)
		return map[string]string{
			"format": "png",
			"data":   "data:image/png;base64,aGVsbG8=", // "hello" in base64
		}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	data, err := client.TakeScreenshot()
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), data)
}

// --- Source tests ---

func TestGetSourceElements(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.dump.ui", method)
		return map[string]interface{}{
			"depth": 3,
			"axElement": map[string]interface{}{
				"identifier":       "",
				"frame":            map[string]float64{"X": 0, "Y": 0, "Width": 390, "Height": 844},
				"label":            "",
				"elementType":      0,
				"enabled":          true,
				"placeholderValue": nil,
				"value":            nil,
				"title":            nil,
				"children": []interface{}{
					map[string]interface{}{
						"identifier":  "submitBtn",
						"frame":       map[string]float64{"X": 100, "Y": 200, "Width": 80, "Height": 44},
						"label":       "Submit",
						"elementType": 9, // Button
						"enabled":     true,
						"children":    []interface{}{},
					},
					map[string]interface{}{
						"identifier":  "nameField",
						"frame":       map[string]float64{"X": 50, "Y": 100, "Width": 300, "Height": 40},
						"label":       "Name",
						"elementType": 48, // TextField
						"enabled":     true,
						"children":    []interface{}{},
					},
					map[string]interface{}{
						"identifier":  "",
						"frame":       map[string]float64{"X": 0, "Y": 0, "Width": 390, "Height": 844},
						"label":       "",
						"elementType": 2, // Other (not in accepted types)
						"enabled":     true,
						"children":    []interface{}{},
					},
					map[string]interface{}{
						"identifier":  "offscreen",
						"frame":       map[string]float64{"X": -10, "Y": -10, "Width": 80, "Height": 44},
						"label":       "Hidden",
						"elementType": 9,
						"enabled":     true,
						"children":    []interface{}{},
					},
					map[string]interface{}{
						"identifier":  "disabled",
						"frame":       map[string]float64{"X": 100, "Y": 300, "Width": 80, "Height": 44},
						"label":       "Disabled",
						"elementType": 9,
						"enabled":     false,
						"children":    []interface{}{},
					},
				},
			},
		}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	elements, err := client.GetSourceElements()
	require.NoError(t, err)

	// should get Button "Submit" and TextField "Name"
	// not: Other (wrong type), offscreen (negative coords), disabled
	require.Len(t, elements, 2)

	assert.Equal(t, "Button", elements[0].Type)
	assert.Equal(t, "Submit", *elements[0].Label)
	assert.Equal(t, "submitBtn", *elements[0].Identifier)
	assert.Equal(t, 100, elements[0].Rect.X)
	assert.Equal(t, 200, elements[0].Rect.Y)

	assert.Equal(t, "TextField", elements[1].Type)
	assert.Equal(t, "Name", *elements[1].Label)
	assert.Equal(t, "nameField", *elements[1].Identifier)
}

func TestGetSourceElements_AlwaysIncludeButtonsWithoutLabel(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		return map[string]interface{}{
			"depth": 2,
			"axElement": map[string]interface{}{
				"identifier": "", "frame": map[string]float64{"X": 0, "Y": 0, "Width": 390, "Height": 844},
				"label": "", "elementType": 0, "enabled": true, "children": []interface{}{
					map[string]interface{}{
						"identifier": "", "frame": map[string]float64{"X": 10, "Y": 10, "Width": 50, "Height": 50},
						"label": "", "elementType": 9, "enabled": true, "children": []interface{}{},
					},
					map[string]interface{}{
						"identifier": "", "frame": map[string]float64{"X": 10, "Y": 100, "Width": 50, "Height": 50},
						"label": "", "elementType": 52, "enabled": true, "children": []interface{}{}, // StaticText without label
					},
				},
			},
		}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	elements, err := client.GetSourceElements()
	require.NoError(t, err)

	// Button always included, StaticText without label/identifier is filtered out
	require.Len(t, elements, 1)
	assert.Equal(t, "Button", elements[0].Type)
}

func TestGetSourceRaw(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		return map[string]interface{}{
			"depth":     1,
			"axElement": map[string]interface{}{"identifier": "root"},
		}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	raw, err := client.GetSourceRaw()
	require.NoError(t, err)
	assert.NotNil(t, raw)

	m := raw.(map[string]interface{})
	assert.NotNil(t, m["axElement"])
}

// --- Orientation tests ---

func TestGetOrientation(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantResult string
	}{
		{"portrait", "PORTRAIT", "portrait"},
		{"landscape", "LANDSCAPE", "landscape"},
		{"unknown defaults to portrait", "UNKNOWN", "portrait"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
				assert.Equal(t, "device.io.orientation.get", method)
				return map[string]string{"orientation": tt.response}, nil
			})
			defer server.Close()

			client := newTestClient(server)
			defer client.Close()

			result, err := client.GetOrientation()
			require.NoError(t, err)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestSetOrientation(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.io.orientation.set", method)
		var p map[string]interface{}
		require.NoError(t, json.Unmarshal(params, &p))
		assert.Equal(t, "LANDSCAPE", p["orientation"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.SetOrientation("landscape")
	assert.NoError(t, err)
}

func TestSetOrientation_Invalid(t *testing.T) {
	client := NewClient("localhost", 1)
	err := client.SetOrientation("diagonal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid orientation")
}

// --- WindowSize tests ---

func TestGetWindowSize(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.info", method)
		return map[string]interface{}{
			"screenSize": map[string]float64{"width": 390, "height": 844},
			"scale":      3,
		}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	size, err := client.GetWindowSize()
	require.NoError(t, err)
	assert.Equal(t, 390, size.ScreenSize.Width)
	assert.Equal(t, 844, size.ScreenSize.Height)
	assert.Equal(t, 3, size.Scale)
}

// --- ForegroundApp tests ---

func TestGetForegroundApp(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.apps.foreground", method)
		return map[string]string{
			"bundleId": "com.example.app",
			"name":     "Example",
		}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	app, err := client.GetForegroundApp()
	require.NoError(t, err)
	assert.Equal(t, "com.example.app", app.BundleID)
	assert.Equal(t, "Example", app.Name)
}

// --- App control tests ---

func TestLaunchApp(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.apps.launch", method)
		var p map[string]interface{}
		json.Unmarshal(params, &p)
		assert.Equal(t, "com.example.app", p["bundleId"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.LaunchApp("com.example.app")
	assert.NoError(t, err)
}

func TestTerminateApp(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		assert.Equal(t, "device.apps.terminate", method)
		var p map[string]interface{}
		json.Unmarshal(params, &p)
		assert.Equal(t, "com.example.app", p["bundleId"])
		return map[string]bool{"success": true}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	err := client.TerminateApp("com.example.app")
	assert.NoError(t, err)
}

// --- Element type mapping tests ---

func TestElementTypeMap(t *testing.T) {
	expected := map[int]string{
		9: "Button", 40: "Switch", 48: "TextField",
		49: "SearchField", 52: "StaticText", 57: "Icon", 69: "Image",
	}
	for k, v := range expected {
		assert.Equal(t, v, elementTypeMap[k])
	}

	_, exists := elementTypeMap[0]
	assert.False(t, exists)
	_, exists = elementTypeMap[999]
	assert.False(t, exists)
}

// --- Concurrent calls test ---

func TestConcurrentCalls(t *testing.T) {
	server := newMockServer(func(method string, params json.RawMessage) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return map[string]string{"method": method}, nil
	})
	defer server.Close()

	client := newTestClient(server)
	defer client.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.call("device.io.tap", map[string]interface{}{"x": 100, "y": 200, "deviceId": ""})
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent call failed: %v", err)
	}
}

func TestNewClient_PortFormatting(t *testing.T) {
	c := NewClient("127.0.0.1", 12004)
	assert.Equal(t, "http://127.0.0.1:12004", c.httpURL)
	assert.Equal(t, "ws://127.0.0.1:12004", c.wsURL)
}
