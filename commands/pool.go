package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
	"github.com/mobile-next/mobilecli/devices"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type PoolAllocateRequest struct {
	Platform string
	Token    string
}

type PoolAllocateDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	Model    string `json:"model"`
}

type PoolAllocateResponse struct {
	SessionID string             `json:"sessionId"`
	Device    PoolAllocateDevice `json:"device"`
}

const defaultPoolServerURL = "ws://localhost:15000/ws"

func getPoolServerURL() string {
	if url := os.Getenv("MOBILECLI_POOL_URL"); url != "" {
		return url
	}
	return defaultPoolServerURL
}

func dialPool(token string) (*websocket.Conn, error) {
	url := getPoolServerURL() + "?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	return conn, err
}

func PoolAllocateCommand(req PoolAllocateRequest) *CommandResponse {
	conn, err := dialPool(req.Token)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to connect to pool server: %w", err))
	}
	defer conn.Close()

	params, err := json.Marshal(map[string]string{"platform": req.Platform})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to marshal params: %w", err))
	}

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pool.allocate",
		Params:  params,
	}

	if err := conn.WriteJSON(rpcReq); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to send request: %w", err))
	}

	var rpcResp jsonRPCResponse
	if err := conn.ReadJSON(&rpcResp); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to read response: %w", err))
	}

	if rpcResp.Error != nil {
		return NewErrorResponse(fmt.Errorf("pool.allocate failed: %v", rpcResp.Error))
	}

	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to marshal result: %w", err))
	}

	var allocateResp PoolAllocateResponse
	if err := json.Unmarshal(resultBytes, &allocateResp); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to parse allocate response: %w", err))
	}

	return NewSuccessResponse(allocateResp)
}

// FetchRemoteDevices fetches devices from the remote pool server via devices.list JSON-RPC
func FetchRemoteDevices(token string) ([]devices.DeviceInfo, error) {
	conn, err := dialPool(token)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to pool server: %w", err)
	}
	defer conn.Close()

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "devices.list",
	}

	if err := conn.WriteJSON(rpcReq); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	var rpcResp jsonRPCResponse
	if err := conn.ReadJSON(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("devices.list failed: %v", rpcResp.Error)
	}

	resultBytes, err := json.Marshal(rpcResp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var result struct {
		Devices []devices.DeviceInfo `json:"devices"`
	}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse devices response: %w", err)
	}

	for i := range result.Devices {
		result.Devices[i].Remote = true
	}

	return result.Devices, nil
}

type PoolReleaseRequest struct {
	DeviceID string
	Token    string
}

func PoolReleaseCommand(req PoolReleaseRequest) *CommandResponse {
	conn, err := dialPool(req.Token)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to connect to pool server: %w", err))
	}
	defer conn.Close()

	params, err := json.Marshal(map[string]string{"deviceId": req.DeviceID})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to marshal params: %w", err))
	}

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pool.release",
		Params:  params,
	}

	if err := conn.WriteJSON(rpcReq); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to send request: %w", err))
	}

	var rpcResp jsonRPCResponse
	if err := conn.ReadJSON(&rpcResp); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to read response: %w", err))
	}

	if rpcResp.Error != nil {
		return NewErrorResponse(fmt.Errorf("pool.release failed: %v", rpcResp.Error))
	}

	return NewSuccessResponse(rpcResp.Result)
}

type PoolListRequest struct {
	Token string
}

func PoolListCommand(req PoolListRequest) *CommandResponse {
	conn, err := dialPool(req.Token)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to connect to pool server: %w", err))
	}
	defer conn.Close()

	rpcReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "pool.list",
	}

	if err := conn.WriteJSON(rpcReq); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to send request: %w", err))
	}

	var rpcResp jsonRPCResponse
	if err := conn.ReadJSON(&rpcResp); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to read response: %w", err))
	}

	if rpcResp.Error != nil {
		return NewErrorResponse(fmt.Errorf("pool.list failed: %v", rpcResp.Error))
	}

	return NewSuccessResponse(rpcResp.Result)
}
