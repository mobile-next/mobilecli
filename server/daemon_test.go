package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noNotify(any) error { return nil }

func TestDaemonDispatchReturnsFullEnvelopeForCLIMethods(t *testing.T) {
	result, err := DaemonDispatch(context.Background(), "cli.io.tap", json.RawMessage(`{"deviceId":"__nope__","x":1,"y":2}`), noNotify)

	require.NoError(t, err, "cli methods report failures inside the envelope, not as rpc errors")
	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"status":"error"`)
	assert.Contains(t, string(data), "__nope__")
}

func TestDaemonDispatchFallsBackToJSONRPCRegistry(t *testing.T) {
	_, err := DaemonDispatch(context.Background(), "device.io.tap", json.RawMessage(`{"deviceId":"__nope__","x":1,"y":2}`), noNotify)

	require.Error(t, err, "json-rpc methods surface failures as rpc errors")
	assert.Contains(t, err.Error(), "__nope__")
}

func TestDaemonDispatchRejectsUnknownMethod(t *testing.T) {
	_, err := DaemonDispatch(context.Background(), "nope.nope", nil, noNotify)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDaemonDispatchRefusesHTTPSessionMethods(t *testing.T) {
	// session tickets are minted by the http front, never by the daemon
	for _, m := range []string{"device.logs", "device.screencapture", "server.shutdown"} {
		_, err := DaemonDispatch(context.Background(), m, json.RawMessage(`{}`), noNotify)
		require.Error(t, err, m)
		assert.Contains(t, err.Error(), "not found", m)
	}
}

func TestCLIStreamMethodsReportUnknownDeviceInsideEnvelope(t *testing.T) {
	for _, m := range []string{"cli.device.logs", "cli.screencapture", "cli.screenrecord"} {
		result, err := DaemonDispatch(context.Background(), m, json.RawMessage(`{"deviceId":"__nope__","format":"mjpeg","output":"/tmp/x.mp4"}`), noNotify)
		require.NoError(t, err, m)
		data, err := json.Marshal(result)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"status":"error"`, m)
		assert.Contains(t, string(data), "__nope__", m)
	}
}

func TestLineNotifierSendsOneNotificationPerLine(t *testing.T) {
	var got []string
	w := &lineNotifier{notify: func(v any) error {
		got = append(got, string(v.(json.RawMessage)))
		return nil
	}}

	_, err := w.Write([]byte("{\"a\":1}\n{\"b\":2}\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("{\"c\":"))
	require.NoError(t, err)
	_, err = w.Write([]byte("3}\n"))
	require.NoError(t, err)

	assert.Equal(t, []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}, got)
}

func TestMessageNotifierWrapsWritesAsMessages(t *testing.T) {
	var got []any
	w := &messageNotifier{notify: func(v any) error { got = append(got, v); return nil }}

	_, err := w.Write([]byte("\rRecording 00:01"))
	require.NoError(t, err)

	assert.Equal(t, []any{map[string]string{"progress": "\rRecording 00:01"}}, got)
}
