package rpc

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
)

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      int         `json:"id"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

const defaultPoolServerURL = "wss://api.mobilenexthq.com/ws"

func GetPoolServerURL() string {
	if url := os.Getenv("MOBILECLI_POOL_URL"); url != "" {
		return url
	}
	return defaultPoolServerURL
}

func Dial(token string) (*websocket.Conn, error) {
	url := GetPoolServerURL() + "?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	return conn, err
}

// Call dials the pool server, sends a JSON-RPC request, and unmarshals the result.
// if result is nil, the response result is discarded.
func Call(token, method string, params interface{}, result interface{}) error {
	conn, err := Dial(token)
	if err != nil {
		return fmt.Errorf("failed to connect to pool server: %w", err)
	}
	defer conn.Close()

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	if err := conn.WriteJSON(req); err != nil {
		return fmt.Errorf("failed to send rpc request: %w", err)
	}

	var resp Response
	if err := conn.ReadJSON(&resp); err != nil {
		return fmt.Errorf("failed to read rpc response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("%s failed: %v", method, resp.Error)
	}

	if result != nil {
		return Remarshal(resp.Result, result)
	}

	return nil
}

// Remarshal converts an interface{} to a typed struct via json round-trip
func Remarshal(src interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal rpc result: %w", err)
	}
	return json.Unmarshal(data, dst)
}
