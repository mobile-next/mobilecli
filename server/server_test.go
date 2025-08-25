package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testServerURL  = "http://localhost:12001"
	testServerPort = "12001"
	serverTimeout  = 8 * time.Second
)

var serverProcess *exec.Cmd

// TestMain starts the server before running tests and cleans up after
func TestMain(m *testing.M) {
	// Start the mobilecli server
	if err := startTestServer(); err != nil {
		fmt.Printf("Failed to start test server: %v\n", err)
		os.Exit(1)
	}

	// Wait for server to be ready
	if err := waitForServer(testServerURL, serverTimeout); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
		stopTestServer()
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Clean up
	stopTestServer()
	os.Exit(code)
}

// startTestServer starts the mobilecli server process
func startTestServer() error {
	// Look for mobilecli binary in parent directory
	serverProcess = exec.Command("../mobilecli", "server", "start", "--listen", "localhost:"+testServerPort)
	serverProcess.Stdout = nil // Suppress output
	serverProcess.Stderr = nil // Suppress output

	return serverProcess.Start()
}

// stopTestServer stops the server process
func stopTestServer() {
	if serverProcess != nil && serverProcess.Process != nil {
		serverProcess.Process.Kill()
		serverProcess.Wait()
	}
}

// waitForServer polls the server until it responds or times out
func waitForServer(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server did not start within %v", timeout)
		case <-ticker.C:
			resp, err := http.Get(url)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// TestRootEndpoint tests that the root endpoint returns status "ok"
func TestRootEndpoint(t *testing.T) {
	resp, err := http.Get(testServerURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var data map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&data))

	assert.Equal(t, "ok", data["status"])
}

// TestRPCEndpointMethods tests HTTP method handling for /rpc endpoint
func TestRPCEndpointMethods(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "GET should return 405 Method Not Allowed",
			method:         "GET",
			expectedStatus: 405,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, testServerURL+"/rpc", nil)
			require.NoError(t, err)

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

// TestJSONRPCValidation tests JSON-RPC request validation
func TestJSONRPCValidation(t *testing.T) {
	tests := []struct {
		name          string
		payload       interface{}
		expectedError map[string]interface{}
		description   string
	}{
		{
			name:    "Empty POST body should return parse error",
			payload: "",
			expectedError: map[string]interface{}{
				"code": float64(ErrCodeParseError),
				"data": "expecting jsonrpc payload",
			},
			description: "POST with empty body",
		},
		{
			name: "Invalid jsonrpc version should return error",
			payload: map[string]interface{}{
				"jsonrpc": "1.0",
				"method":  "devices",
				"id":      1,
			},
			expectedError: map[string]interface{}{
				"code": float64(ErrCodeInvalidRequest),
				"data": "'jsonrpc' must be '2.0'",
			},
			description: "jsonrpc version 1.0",
		},
		{
			name: "Missing id field should return error",
			payload: map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "devices",
				"params":  map[string]interface{}{},
			},
			expectedError: map[string]interface{}{
				"code": float64(ErrCodeInvalidRequest),
				"data": "'id' field is required",
			},
			description: "Missing id field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if tt.payload == "" {
				body = []byte("")
			} else {
				body, err = json.Marshal(tt.payload)
				require.NoError(t, err)
			}

			resp, err := http.Post(testServerURL+"/rpc", "application/json", bytes.NewBuffer(body))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)

			var jsonResp JSONRPCResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&jsonResp))

			assert.Equal(t, "2.0", jsonResp.JSONRPC)
			assert.NotNil(t, jsonResp.Error, "Expected error in response")

			errorMap, ok := jsonResp.Error.(map[string]interface{})
			require.True(t, ok, "Expected error to be map, got %T", jsonResp.Error)

			assert.Equal(t, tt.expectedError["code"], errorMap["code"])
			assert.Equal(t, tt.expectedError["data"], errorMap["data"])
		})
	}
}

// TestDeviceInfoRequiredParams tests that device_info method requires params
func TestDeviceInfoRequiredParams(t *testing.T) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "device_info",
		"id":      1,
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(testServerURL+"/rpc", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var jsonResp JSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jsonResp))

	assert.Equal(t, "2.0", jsonResp.JSONRPC)
	assert.Equal(t, float64(1), jsonResp.ID)
	assert.NotNil(t, jsonResp.Error, "Expected error in response")

	errorMap, ok := jsonResp.Error.(map[string]interface{})
	require.True(t, ok, "Expected error to be map, got %T", jsonResp.Error)

	assert.Equal(t, float64(ErrCodeServerError), errorMap["code"])
	assert.Equal(t, "'params' is required with fields: deviceId", errorMap["data"])
}

// TestMethodNotFound tests that unknown methods return method not found error
func TestMethodNotFound(t *testing.T) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "unknown_method",
		"id":      1,
	}

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(testServerURL+"/rpc", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var jsonResp JSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jsonResp))

	assert.NotNil(t, jsonResp.Error, "Expected error in response")

	errorMap, ok := jsonResp.Error.(map[string]interface{})
	require.True(t, ok, "Expected error to be map, got %T", jsonResp.Error)

	assert.Equal(t, float64(ErrCodeMethodNotFound), errorMap["code"])
}

// TestMissingMethod tests that missing method field returns error
func TestMissingMethod(t *testing.T) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	resp, err := http.Post(testServerURL+"/rpc", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	var jsonResp JSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jsonResp))

	assert.NotNil(t, jsonResp.Error, "Expected error in response")

	errorMap, ok := jsonResp.Error.(map[string]interface{})
	require.True(t, ok, "Expected error to be map, got %T", jsonResp.Error)

	assert.Equal(t, float64(ErrCodeServerError), errorMap["code"])
	assert.Equal(t, "'method' is required", errorMap["data"])
}

// Unit tests for better code coverage

// TestSendBanner tests the banner/root endpoint handler directly
func TestSendBanner(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	sendBanner(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if data["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", data["status"])
	}
}

// TestHandleJSONRPCDirect tests the JSON-RPC handler directly
func TestHandleJSONRPCDirect(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		body         string
		expectStatus int
		expectError  bool
	}{
		{
			name:         "Non-POST method",
			method:       "GET",
			body:         "",
			expectStatus: 405,
			expectError:  false,
		},
		{
			name:         "Valid JSON-RPC request with unknown method",
			method:       "POST",
			body:         `{"jsonrpc":"2.0","method":"unknown","id":1}`,
			expectStatus: 200,
			expectError:  true,
		},
		{
			name:         "Invalid JSON",
			method:       "POST",
			body:         `{invalid json}`,
			expectStatus: 200,
			expectError:  true,
		},
		{
			name:         "Empty method",
			method:       "POST",
			body:         `{"jsonrpc":"2.0","method":"","id":1}`,
			expectStatus: 200,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/rpc", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			handleJSONRPC(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.expectStatus, resp.StatusCode)

			// For 405 responses, there won't be JSON content
			if resp.StatusCode == 405 {
				return
			}

			var jsonResp JSONRPCResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&jsonResp))

			if tt.expectError {
				assert.NotNil(t, jsonResp.Error, "Expected error in response")
			} else {
				assert.Nil(t, jsonResp.Error, "Expected no error in response")
			}
		})
	}
}

// TestSendJSONRPCResponse tests the response helper function
func TestSendJSONRPCResponse(t *testing.T) {
	w := httptest.NewRecorder()
	testData := map[string]string{"test": "data"}

	sendJSONRPCResponse(w, 123, testData)

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp JSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jsonResp))

	assert.Equal(t, "2.0", jsonResp.JSONRPC)
	assert.Equal(t, float64(123), jsonResp.ID)

	resultMap, ok := jsonResp.Result.(map[string]interface{})
	require.True(t, ok, "Expected result to be map, got %T", jsonResp.Result)

	assert.Equal(t, "data", resultMap["test"])
}

// TestSendJSONRPCError tests the error response helper function
func TestSendJSONRPCError(t *testing.T) {
	w := httptest.NewRecorder()

	sendJSONRPCError(w, 456, ErrCodeMethodNotFound, "Method not found", "Test method")

	resp := w.Result()
	defer resp.Body.Close()

	var jsonResp JSONRPCResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&jsonResp))

	assert.Equal(t, "2.0", jsonResp.JSONRPC)
	assert.Equal(t, float64(456), jsonResp.ID)

	errorMap, ok := jsonResp.Error.(map[string]interface{})
	require.True(t, ok, "Expected error to be map, got %T", jsonResp.Error)

	assert.Equal(t, float64(ErrCodeMethodNotFound), errorMap["code"])
	assert.Equal(t, "Method not found", errorMap["message"])
	assert.Equal(t, "Test method", errorMap["data"])
}

// TestCORSMiddleware tests the CORS middleware functionality
func TestCORSMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	corsHandler := corsMiddleware(testHandler)

	tests := []struct {
		name   string
		method string
	}{
		{"GET request", "GET"},
		{"POST request", "POST"},
		{"OPTIONS request", "OPTIONS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			w := httptest.NewRecorder()

			corsHandler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			// Check CORS headers
			assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
			assert.Equal(t, "POST, GET, OPTIONS", resp.Header.Get("Access-Control-Allow-Methods"))

			// OPTIONS requests should return 200 without calling the handler
			if tt.method == "OPTIONS" {
				assert.Equal(t, 200, resp.StatusCode)
			}
		})
	}
}

// TestInvalidServerCommand tests that invalid server command usage returns an error
func TestInvalidServerCommand(t *testing.T) {
	// Test invalid command format: "mobilecli server start 12000" (arguments not allowed)
	cmd := exec.Command("../mobilecli", "server", "start", "12000")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Should return an error since arguments are not allowed after "start"
	assert.Error(t, err, "Expected error for invalid command format")

	// Check that stderr contains error message about unknown command
	stderrStr := stderr.String()
	assert.Contains(t, stderrStr, "unknown command \"12000\" for \"mobilecli server start\"", "Expected error message about unknown command")
}
