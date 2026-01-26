package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(enableCORS bool) (*httptest.Server, string) {
	handler := NewWebSocketHandler(enableCORS)
	server := httptest.NewServer(handler)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	return server, wsURL
}

func connectWebSocket(t *testing.T, url string) *websocket.Conn {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err, "should connect to WebSocket")
	return conn
}

func sendJSONRPCRequest(t *testing.T, conn *websocket.Conn, req JSONRPCRequest) {
	err := conn.WriteJSON(req)
	require.NoError(t, err, "should send request")
}

func readJSONRPCResponse(t *testing.T, conn *websocket.Conn) JSONRPCResponse {
	var resp JSONRPCResponse
	err := conn.ReadJSON(&resp)
	require.NoError(t, err, "should read response")
	return resp
}

func TestWebSocket_ValidRequest(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, 1, int(resp.ID.(float64)))
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestWebSocket_MissingJSONRPCVersion(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "1.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeInvalidRequest), errorMap["code"])
	assert.Equal(t, errTitleInvalidReq, errorMap["message"])
	assert.Equal(t, errMsgInvalidJSONRPC, errorMap["data"])
}

func TestWebSocket_MissingID(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      nil,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeInvalidRequest), errorMap["code"])
	assert.Equal(t, errTitleInvalidReq, errorMap["message"])
	assert.Equal(t, errMsgIDRequired, errorMap["data"])
}

func TestWebSocket_MissingMethod(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeInvalidRequest), errorMap["code"])
	assert.Equal(t, errTitleInvalidReq, errorMap["message"])
	assert.Equal(t, errMsgMethodRequired, errorMap["data"])
}

func TestWebSocket_ScreencaptureNotSupported(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "screencapture",
		Params:  json.RawMessage(`{"deviceId":"test"}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeMethodNotFound), errorMap["code"])
	assert.Equal(t, errTitleMethodNotSupp, errorMap["message"])
	assert.Equal(t, errMsgScreencapture, errorMap["data"])
}

func TestWebSocket_MethodNotFound(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "nonexistent_method",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeMethodNotFound), errorMap["code"])
	assert.Equal(t, "Method not found", errorMap["message"])
}

func TestWebSocket_InvalidJSON(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	err := conn.WriteMessage(websocket.TextMessage, []byte("invalid json"))
	require.NoError(t, err)

	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeParseError), errorMap["code"])
	assert.Equal(t, errTitleParseError, errorMap["message"])
	assert.Equal(t, errMsgParseError, errorMap["data"])
}

func TestWebSocket_BinaryMessageRejected(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	err := conn.WriteMessage(websocket.BinaryMessage, []byte("binary data"))
	require.NoError(t, err)

	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.NotNil(t, resp.Error)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeInvalidRequest), errorMap["code"])
	assert.Equal(t, errTitleInvalidReq, errorMap["message"])
	assert.Equal(t, errMsgTextOnly, errorMap["data"])
}

func TestWebSocket_MultipleRequests(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	for i := 1; i <= 3; i++ {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "devices",
			Params:  json.RawMessage(`{}`),
			ID:      i,
		}

		sendJSONRPCRequest(t, conn, req)
		resp := readJSONRPCResponse(t, conn)

		assert.Equal(t, "2.0", resp.JSONRPC)
		assert.Equal(t, i, int(resp.ID.(float64)))
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Result)
	}
}

func TestWebSocket_PingPong(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	pongReceived := make(chan bool, 1)

	conn.SetPongHandler(func(appData string) error {
		pongReceived <- true
		return nil
	})

	// send ping from client side
	err := conn.WriteMessage(websocket.PingMessage, nil)
	require.NoError(t, err)

	// start reading to process pong
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// wait for pong with timeout
	select {
	case <-pongReceived:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive pong response")
	}
}

func TestWebSocket_CORSEnabled(t *testing.T) {
	server, wsURL := setupTestServer(true)
	defer server.Close()

	// connect with different origin
	headers := http.Header{}
	headers.Set("Origin", "http://different-origin.com")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	require.NoError(t, err, "should connect with CORS enabled")
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Nil(t, resp.Error)
}

func TestWebSocket_CORSDisabled(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	// try to connect with different origin
	headers := http.Header{}
	headers.Set("Origin", "http://different-origin.com")

	_, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	assert.Error(t, err, "should reject connection with different origin when CORS disabled")
}

func TestWebSocket_ConnectionLifecycle(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)
	assert.Nil(t, resp.Error)

	// close connection
	err := conn.Close()
	require.NoError(t, err)

	// attempt to send after close should fail
	err = conn.WriteJSON(req)
	assert.Error(t, err, "should fail to send after connection closed")
}

func TestWebSocket_StringID(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      "string-id-123",
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "string-id-123", resp.ID)
	assert.Nil(t, resp.Error)
}

func TestValidateJSONRPCRequest_AllValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		req      JSONRPCRequest
		wantCode int
		wantMsg  string
		wantData string
	}{
		{
			name: "invalid jsonrpc version",
			req: JSONRPCRequest{
				JSONRPC: "1.0",
				Method:  "devices",
				ID:      1,
			},
			wantCode: ErrCodeInvalidRequest,
			wantMsg:  errTitleInvalidReq,
			wantData: errMsgInvalidJSONRPC,
		},
		{
			name: "missing id",
			req: JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "devices",
				ID:      nil,
			},
			wantCode: ErrCodeInvalidRequest,
			wantMsg:  errTitleInvalidReq,
			wantData: errMsgIDRequired,
		},
		{
			name: "missing method",
			req: JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "",
				ID:      1,
			},
			wantCode: ErrCodeInvalidRequest,
			wantMsg:  errTitleInvalidReq,
			wantData: errMsgMethodRequired,
		},
		{
			name: "screencapture method",
			req: JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "screencapture",
				ID:      1,
			},
			wantCode: ErrCodeMethodNotFound,
			wantMsg:  errTitleMethodNotSupp,
			wantData: errMsgScreencapture,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONRPCRequest(tt.req)
			require.NotNil(t, err, "should return validation error")
			assert.Equal(t, tt.wantCode, err.code)
			assert.Equal(t, tt.wantMsg, err.message)
			assert.Equal(t, tt.wantData, err.data)
		})
	}
}

func TestValidateJSONRPCRequest_Valid(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		ID:      1,
	}

	err := validateJSONRPCRequest(req)
	assert.Nil(t, err, "should not return error for valid request")
}

func TestNewUpgrader_CORSEnabled(t *testing.T) {
	upgrader := newUpgrader(true)
	assert.NotNil(t, upgrader)
	assert.NotNil(t, upgrader.CheckOrigin)

	// test that CheckOrigin allows any origin
	req := &http.Request{
		Header: http.Header{},
	}
	req.Header.Set("Origin", "http://any-origin.com")
	assert.True(t, upgrader.CheckOrigin(req))
}

func TestNewUpgrader_CORSDisabled(t *testing.T) {
	upgrader := newUpgrader(false)
	assert.NotNil(t, upgrader)
	assert.NotNil(t, upgrader.CheckOrigin)
}

func TestIsSameOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		host     string
		expected bool
	}{
		{
			name:     "no origin header",
			origin:   "",
			host:     "localhost:8080",
			expected: true,
		},
		{
			name:     "same origin",
			origin:   "http://localhost:8080",
			host:     "localhost:8080",
			expected: true,
		},
		{
			name:     "different origin",
			origin:   "http://other.com",
			host:     "localhost:8080",
			expected: false,
		},
		{
			name:     "invalid origin url",
			origin:   "://invalid",
			host:     "localhost:8080",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: http.Header{},
				Host:   tt.host,
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			result := isSameOrigin(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebSocket_ConcurrentConnections(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	numConnections := 5
	done := make(chan bool, numConnections)

	for i := 0; i < numConnections; i++ {
		go func(id int) {
			conn := connectWebSocket(t, wsURL)
			defer conn.Close()

			req := JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "devices",
				Params:  json.RawMessage(`{}`),
				ID:      id,
			}

			sendJSONRPCRequest(t, conn, req)
			resp := readJSONRPCResponse(t, conn)

			assert.Equal(t, "2.0", resp.JSONRPC)
			assert.Equal(t, id, int(resp.ID.(float64)))
			assert.Nil(t, resp.Error)

			done <- true
		}(i)
	}

	// wait for all connections to complete
	for i := 0; i < numConnections; i++ {
		select {
		case <-done:
			// success
		case <-time.After(5 * time.Second):
			t.Fatalf("connection %d timed out", i)
		}
	}
}

func TestWebSocket_ReadDeadline(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	// send request to establish connection
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)
	assert.Nil(t, resp.Error)

	// the server should be sending pings periodically
	// we can verify the connection stays alive by sending another request
	time.Sleep(time.Second)

	req.ID = 2
	sendJSONRPCRequest(t, conn, req)
	resp = readJSONRPCResponse(t, conn)
	assert.Equal(t, 2, int(resp.ID.(float64)))
	assert.Nil(t, resp.Error)
}

func BenchmarkWebSocket_SingleRequest(b *testing.B) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.ID = i
		if err := conn.WriteJSON(req); err != nil {
			b.Fatalf("failed to send: %v", err)
		}

		var resp JSONRPCResponse
		if err := conn.ReadJSON(&resp); err != nil {
			b.Fatalf("failed to read: %v", err)
		}
	}
}

func BenchmarkWebSocket_ConcurrentRequests(b *testing.B) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	numWorkers := 10
	done := make(chan bool)

	worker := func() {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			b.Errorf("failed to connect: %v", err)
			return
		}
		defer conn.Close()

		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "devices",
			Params:  json.RawMessage(`{}`),
			ID:      1,
		}

		for i := 0; i < b.N/numWorkers; i++ {
			req.ID = i
			if err := conn.WriteJSON(req); err != nil {
				b.Errorf("failed to send: %v", err)
				return
			}

			var resp JSONRPCResponse
			if err := conn.ReadJSON(&resp); err != nil {
				b.Errorf("failed to read: %v", err)
				return
			}
		}

		done <- true
	}

	b.ResetTimer()
	for i := 0; i < numWorkers; i++ {
		go worker()
	}

	for i := 0; i < numWorkers; i++ {
		<-done
	}
}

func TestWSConnection_SendError(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	// trigger an error by sending invalid method
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "invalid_method",
		Params:  json.RawMessage(`{}`),
		ID:      "test-id",
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "test-id", resp.ID)
	assert.NotNil(t, resp.Error)
	assert.Nil(t, resp.Result)

	errorMap := resp.Error.(map[string]interface{})
	assert.Equal(t, float64(ErrCodeMethodNotFound), errorMap["code"])
	assert.Contains(t, fmt.Sprint(errorMap["message"]), "Method not found")
}

func TestWSConnection_SendResponse(t *testing.T) {
	server, wsURL := setupTestServer(false)
	defer server.Close()

	conn := connectWebSocket(t, wsURL)
	defer conn.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "devices",
		Params:  json.RawMessage(`{}`),
		ID:      "response-test",
	}

	sendJSONRPCRequest(t, conn, req)
	resp := readJSONRPCResponse(t, conn)

	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, "response-test", resp.ID)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}
