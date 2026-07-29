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

// fleetAllocateParams is the params object of the fleet.allocate rpc
type fleetAllocateParams struct {
	Filters []DeviceFilter `json:"filters"`
}

// fleetReleaseParams is the params object of the fleet.release rpc
type fleetReleaseParams struct {
	DeviceID string `json:"deviceId"`
}

// devicesListResult is the result object of the devices.list rpc
type devicesListResult struct {
	Devices []devices.DeviceInfo `json:"devices"`
}

// FleetDevice is a device model offered by the fleet, not an allocated instance
type FleetDevice struct {
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	Type       string `json:"type"`
	Version    string `json:"version"`
	FormFactor string `json:"formFactor"`
}

// FleetListDevicesResponse is the result object of the fleet.listDevices rpc
type FleetListDevicesResponse struct {
	Devices []FleetDevice `json:"devices"`
}

type FleetAllocateRequest struct {
	Filters []DeviceFilter
	Token   string
}

// FleetAllocateResponse is the result object of the fleet.allocate rpc. Device is
// filled in by the caller once allocation completes, the rpc itself never returns it.
type FleetAllocateResponse struct {
	AllocationID string              `json:"allocationId"`
	State        string              `json:"state,omitempty"`
	Device       *devices.DeviceInfo `json:"device,omitempty"`
}

func (r FleetAllocateResponse) IsAllocating() bool {
	return r.State == "allocating"
}

func FleetAllocateCommand(req FleetAllocateRequest) *CommandResponse {
	var result FleetAllocateResponse
	params := fleetAllocateParams{Filters: req.Filters}
	err := rpc.Call(req.Token, "fleet.allocate", params, &result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.allocate: %w", err))
	}
	return NewSuccessResponse(result)
}

// fetches devices from the remote fleet server via devices.list JSON-RPC
func FetchRemoteDevices(token string) ([]devices.DeviceInfo, error) {
	result, err := fleetListAllocatedDevices(token)
	if err != nil {
		return nil, err
	}

	for i := range result {
		// the server already reports its provider (including the allocation id),
		// only fill it in when it's missing so we don't overwrite it
		if len(result[i].Provider) == 0 {
			result[i].SetProvider("mobilenext")
		}
	}

	return result, nil
}

func fleetListAllocatedDevices(token string) ([]devices.DeviceInfo, error) {
	var raw json.RawMessage
	if err := rpc.Call(token, "devices.list", nil, &raw); err != nil {
		return nil, fmt.Errorf("devices.list: %w", err)
	}

	utils.Verbose("remote devices response: %s", string(raw))

	var result devicesListResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("devices.list: %w", err)
	}

	return result.Devices, nil
}

// finds a device by allocation ID using devices.list and returns its info
func FleetGetDeviceByAllocation(token, allocationID string) (*devices.DeviceInfo, error) {
	deviceList, err := fleetListAllocatedDevices(token)
	if err != nil {
		return nil, err
	}

	device := findDeviceByAllocation(deviceList, allocationID)
	if device == nil {
		return nil, fmt.Errorf("device with allocation %s not found", allocationID)
	}

	return device, nil
}

func findDeviceByAllocation(deviceList []devices.DeviceInfo, allocationID string) *devices.DeviceInfo {
	for _, d := range deviceList {
		var p devices.DeviceProvider
		if json.Unmarshal(d.Provider, &p) == nil && p.AllocationID == allocationID {
			return &d
		}
	}

	return nil
}

type FleetReleaseRequest struct {
	DeviceID string
	Token    string
}

func FleetReleaseCommand(req FleetReleaseRequest) *CommandResponse {
	err := rpc.Call(req.Token, "fleet.release", fleetReleaseParams{DeviceID: req.DeviceID}, nil)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.release: %w", err))
	}
	return NewSuccessResponse(nil)
}

type FleetListDevicesRequest struct {
	Token string
}

func FleetListDevicesCommand(req FleetListDevicesRequest) *CommandResponse {
	var result FleetListDevicesResponse
	err := rpc.Call(req.Token, "fleet.listDevices", nil, &result)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.listDevices: %w", err))
	}
	return NewSuccessResponse(result)
}
