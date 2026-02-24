package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  any `json:"params,omitempty"`
	ID      int         `json:"id"`
}

// RPCError represents a JSON-RPC 2.0 error object
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e.Data != "" {
		return e.Data
	}
	return e.Message
}

type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
	ID      any `json:"id"`
}

const defaultFleetServerURL = "wss://api.mobilenexthq.com/ws"

func GetFleetServerURL() string {
	if url := os.Getenv("MOBILECLI_FLEET_URL"); url != "" {
		return url
	}
	return defaultFleetServerURL
}

const rpcTimeout = 30 * time.Second

var fleetDialer = websocket.Dialer{
	HandshakeTimeout: 10 * time.Second,
}

func Dial(token string) (*websocket.Conn, error) {
	u, err := url.Parse(GetFleetServerURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse fleet server URL: %w", err)
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, _, err := fleetDialer.Dial(u.String(), header)
	return conn, err
}

// dials the fleet server, sends a JSON-RPC request, and unmarshals the result.
// if result is nil, the response result is discarded.
func Call(token, method string, params any, result any) error {
	conn, err := Dial(token)
	if err != nil {
		return fmt.Errorf("failed to connect to fleet server: %w", err)
	}
	defer conn.Close()

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	if err := conn.SetWriteDeadline(time.Now().Add(rpcTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	if err := conn.WriteJSON(req); err != nil {
		return fmt.Errorf("failed to send rpc request: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(rpcTimeout)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	var resp Response
	if err := conn.ReadJSON(&resp); err != nil {
		return fmt.Errorf("failed to read rpc response: %w", err)
	}

	if resp.Error != nil {
		return resp.Error
	}

	if result != nil {
		return Remarshal(resp.Result, result)
	}

	return nil
}

// Remarshal converts any value to a typed struct via json round-trip
func Remarshal(src any, dst any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal rpc result: %w", err)
	}
	return json.Unmarshal(data, dst)
}
