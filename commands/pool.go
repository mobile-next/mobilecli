package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/rpc"
)

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

func PoolAllocateCommand(req PoolAllocateRequest) *CommandResponse {
	var result PoolAllocateResponse
	err := rpc.Call(req.Token, "pool.allocate", map[string]string{"platform": req.Platform}, &result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("pool.allocate: %w", err))
	}
	return NewSuccessResponse(result)
}

// FetchRemoteDevices fetches devices from the remote pool server via devices.list JSON-RPC
func FetchRemoteDevices(token string) ([]devices.DeviceInfo, error) {
	var result struct {
		Devices []devices.DeviceInfo `json:"devices"`
	}
	if err := rpc.Call(token, "devices.list", nil, &result); err != nil {
		return nil, err
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
	err := rpc.Call(req.Token, "pool.release", map[string]string{"deviceId": req.DeviceID}, nil)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("pool.release: %w", err))
	}
	return NewSuccessResponse(nil)
}

type PoolListRequest struct {
	Token string
}

func PoolListCommand(req PoolListRequest) *CommandResponse {
	var result any
	err := rpc.Call(req.Token, "pool.list", nil, &result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("pool.list: %w", err))
	}
	return NewSuccessResponse(result)
}
