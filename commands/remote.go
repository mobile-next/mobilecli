package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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

// fleetAllocateParams is the request body of POST /api/v1/sessions/{sessionId}/devices
type fleetAllocateParams struct {
	Filters []DeviceFilter `json:"filters"`
}

// createSessionResponse is the response body of POST /api/v1/sessions
type createSessionResponse struct {
	ID string `json:"id"`
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

// FleetAllocateCommand creates a session and allocates a device into it over REST, so the
// allocation is linked to a sessions row from the start (a WebSocket fleet.allocate call has
// no sessionId, leaving the allocation invisible to GET /api/v1/sessions).
func FleetAllocateCommand(req FleetAllocateRequest) *CommandResponse {
	var session createSessionResponse
	if err := rpc.RESTCall(req.Token, http.MethodPost, "/api/v1/sessions", nil, &session); err != nil {
		return NewErrorResponse(fmt.Errorf("create session: %w", err))
	}

	var result FleetAllocateResponse
	params := fleetAllocateParams{Filters: req.Filters}
	path := fmt.Sprintf("/api/v1/sessions/%s/devices", session.ID)
	if err := rpc.RESTCall(req.Token, http.MethodPost, path, params, &result); err != nil {
		return NewErrorResponse(fmt.Errorf("allocate device: %w", err))
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

// sessionDeviceEntry is a device as embedded in a GET /api/v1/sessions session's "devices" list.
type sessionDeviceEntry struct {
	Status string `json:"status"`
	Info   struct {
		Serial string `json:"serial"`
	} `json:"info"`
}

// sessionListEntry is one session as returned by GET /api/v1/sessions.
type sessionListEntry struct {
	ID      string               `json:"id"`
	Devices []sessionDeviceEntry `json:"devices"`
}

// sessionsPageResponse is the paginated envelope of GET /api/v1/sessions.
type sessionsPageResponse struct {
	Data       []sessionListEntry `json:"data"`
	NextCursor *string            `json:"nextCursor"`
}

// findOwningSessionID pages through the account's sessions to find the one holding a live
// (not yet released) allocation of deviceID. The release endpoint is session-scoped, and
// fleet.release/devices.list never surface a device's owning sessionId, so this is the only
// way to recover it.
func findOwningSessionID(token, deviceID string) (string, error) {
	before := ""
	for {
		path := "/api/v1/sessions?limit=500"
		if before != "" {
			path += "&before=" + url.QueryEscape(before)
		}

		var page sessionsPageResponse
		if err := rpc.RESTCall(token, http.MethodGet, path, nil, &page); err != nil {
			return "", fmt.Errorf("list sessions: %w", err)
		}

		for _, session := range page.Data {
			for _, device := range session.Devices {
				if device.Info.Serial == deviceID && device.Status != "released" {
					return session.ID, nil
				}
			}
		}

		if page.NextCursor == nil {
			return "", fmt.Errorf("device %s is not allocated in any active session", deviceID)
		}
		before = *page.NextCursor
	}
}

func FleetReleaseCommand(req FleetReleaseRequest) *CommandResponse {
	sessionID, err := findOwningSessionID(req.Token, req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("fleet.release: %w", err))
	}

	path := fmt.Sprintf("/api/v1/sessions/%s/devices/%s/release", url.PathEscape(sessionID), url.PathEscape(req.DeviceID))
	if err := rpc.RESTCall(req.Token, http.MethodPost, path, nil, nil); err != nil {
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
