package devices

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// errAgentUnreachable marks a failure to reach an agent at all, as opposed to
// an error the agent itself reported. Callers that can restart their agent use
// it to tell "it died" from "it said no".
var errAgentUnreachable = errors.New("agent unreachable")

// jsonRPCRequest is the envelope every agent call is wrapped in. Params is
// whatever shape the method takes, typically a small struct declared next to
// its caller; nil means the method takes none and the field is left out.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// agentRequest sends a JSON-RPC 2.0 request to the agent over HTTP and returns
// the result field from the response.
func agentRequest(port int, method string, params any) (json.RawMessage, error) {
	return agentRequestWithTimeout(port, method, params, defaultAgentTimeout)
}

func agentRequestWithTimeout(port int, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	payload, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", ID: "1", Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	start := time.Now()
	defer func() {
		utils.Verbose("agentRequest method=%s payloadBytes=%d elapsed=%s", method, len(payload), time.Since(start))
	}()

	client := &http.Client{Timeout: timeout}
	resp, err := postJSON(client, port, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w on port %d: %v", errAgentUnreachable, port, err)
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
	resp, err := postJSON(client, port, bytes.NewReader([]byte("{}")))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func postJSON(client *http.Client, port int, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("http://localhost:%d/", port), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}
