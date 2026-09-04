package devices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

// JSON-RPC 2.0 over HTTP transport shared by every on-device agent reached
// through an adb forward: the persistent DeviceServer, the AvcServer control
// socket and the in-app webview agent.

const defaultAgentTimeout = 10 * time.Second

// agentRequest sends a JSON-RPC 2.0 request to the agent over HTTP and returns
// the result field from the response.
func agentRequest(port int, method string, params map[string]any) (json.RawMessage, error) {
	return agentRequestWithTimeout(port, method, params, defaultAgentTimeout)
}

func agentRequestWithTimeout(port int, method string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  method,
	}
	if len(params) > 0 {
		body["params"] = params
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	start := time.Now()
	defer func() {
		utils.Verbose("agentRequest method=%s payloadBytes=%d elapsed=%s", method, len(payload), time.Since(start))
	}()

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(
		fmt.Sprintf("http://localhost:%d/", port),
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to agent on port %d: %w", port, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read agent response: %w", err)
	}

	var rpc struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &rpc); err != nil {
		return nil, fmt.Errorf("parse agent response: %w", err)
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("agent error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	return rpc.Result, nil
}

// isAgentReady checks whether the agent socket is already accepting connections.
func isAgentReady(port int) bool {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Post(fmt.Sprintf("http://localhost:%d/", port), "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}
