package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/rpc"
)

type FleetAllocateRequest struct {
	Platform string
	Token    string
}

type FleetAllocateDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	Model    string `json:"model"`
}

type FleetAllocateResponse struct {
	SessionID string              `json:"sessionId"`
	Device    FleetAllocateDevice `json:"device"`
}

func FleetAllocateCommand(req FleetAllocateRequest) *CommandResponse {
	var result FleetAllocateResponse
	err := rpc.Call(req.Token, "fleet.allocate", map[string]string{"platform": req.Platform}, &result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.allocate: %w", err))
	}
	return NewSuccessResponse(result)
}

// fetches devices from the remote fleet server via devices.list JSON-RPC
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

type FleetReleaseRequest struct {
	DeviceID string
	Token    string
}

func FleetReleaseCommand(req FleetReleaseRequest) *CommandResponse {
	err := rpc.Call(req.Token, "fleet.release", map[string]string{"deviceId": req.DeviceID}, nil)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.release: %w", err))
	}
	return NewSuccessResponse(nil)
}

type FleetListDevicesRequest struct {
	Token string
}

func FleetListDevicesCommand(req FleetListDevicesRequest) *CommandResponse {
	var result any
	err := rpc.Call(req.Token, "fleet.listDevices", nil, &result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.listDevices: %w", err))
	}
	return NewSuccessResponse(result)
}
