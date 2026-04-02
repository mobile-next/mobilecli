package commands

import (
	"encoding/json"
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/rpc"
	"github.com/mobile-next/mobilecli/utils"
)

// DeviceFilter represents a single filter criterion for device selection.
type DeviceFilter struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     string `json:"value"`
}

type FleetAllocateRequest struct {
	Filters []DeviceFilter
	Token   string
}

type FleetAllocateDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"state"`
	Model    string `json:"model"`
}

type FleetAllocateResponse struct {
	SessionID   string              `json:"sessionId"`
	Device      FleetAllocateDevice `json:"device"`
	ProvisionID string              `json:"provisionId,omitempty"`
	State       string              `json:"state,omitempty"`
}

func (r FleetAllocateResponse) IsAllocating() bool {
	return r.State == "allocating" || r.Device.Status == "allocating"
}

func FleetAllocateCommand(req FleetAllocateRequest) *CommandResponse {
	var result FleetAllocateResponse
	params := map[string]any{
		"filters": req.Filters,
	}
	err := rpc.Call(req.Token, "fleet.allocate", params, &result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.allocate: %w", err))
	}
	return NewSuccessResponse(result)
}

// fetches devices from the remote fleet server via devices.list JSON-RPC
func FetchRemoteDevices(token string) ([]devices.DeviceInfo, error) {
	var raw any
	if err := rpc.Call(token, "devices.list", nil, &raw); err != nil {
		return nil, err
	}

	if rawJSON, err := json.Marshal(raw); err == nil {
		utils.Verbose("remote devices response: %s", string(rawJSON))
	}

	var result struct {
		Devices []devices.DeviceInfo `json:"devices"`
	}
	if err := rpc.Remarshal(raw, &result); err != nil {
		return nil, err
	}

	for i := range result.Devices {
		result.Devices[i].SetProvider("mobilefleet")
	}

	return result.Devices, nil
}

// finds a device by session ID using devices.list and returns its info
func FleetGetDeviceBySession(token, sessionID string) (*devices.DeviceInfo, error) {
	var raw any
	if err := rpc.Call(token, "devices.list", nil, &raw); err != nil {
		return nil, fmt.Errorf("devices.list: %w", err)
	}

	var result struct {
		Devices []devices.DeviceInfo `json:"devices"`
	}
	if err := rpc.Remarshal(raw, &result); err != nil {
		return nil, err
	}

	for _, d := range result.Devices {
		var p devices.DeviceProvider
		if json.Unmarshal(d.Provider, &p) == nil && p.SessionID == sessionID {
			return &d, nil
		}
	}

	return nil, fmt.Errorf("device with session %s not found", sessionID)
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
