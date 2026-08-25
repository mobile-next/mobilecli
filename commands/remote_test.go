package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mobile-next/mobilecli/devices"
)

// devices.list reports the allocation a remote device belongs to in its provider object
func remoteDevice(id, allocationID string) devices.DeviceInfo {
	provider, _ := json.Marshal(devices.DeviceProvider{Type: "mobilenext", AllocationID: allocationID})
	return devices.DeviceInfo{ID: id, Provider: provider}
}

func TestFindDeviceByAllocation(t *testing.T) {
	allocated := remoteDevice("R83X10DZ2TW", "40ec2cb6-1f40-4772-91d9-2c8820f085d0")
	other := remoteDevice("41270DLJG000R7", "ed66ee3a-c33f-40b2-b569-24ca30b2064a")
	local := devices.DeviceInfo{ID: "Pixel_9_Pro"}

	deviceList := []devices.DeviceInfo{local, other, allocated}

	found := findDeviceByAllocation(deviceList, "40ec2cb6-1f40-4772-91d9-2c8820f085d0")
	if found == nil || found.ID != allocated.ID {
		t.Errorf("expected to find %s, got %v", allocated.ID, found)
	}

	if found := findDeviceByAllocation(deviceList, "no-such-allocation"); found != nil {
		t.Errorf("expected no device for unknown allocation, got %s", found.ID)
	}
}

func TestFleetAllocateCommandCreatesASessionThenAllocatesIntoIt(t *testing.T) {
	var gotPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/sessions":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "session-abc"})
		case "/api/v1/sessions/session-abc/devices":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"allocationId": "alloc-123",
				"device":       map[string]string{"id": "R83X10DZ2TW", "platform": "ios"},
			})
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	response := FleetAllocateCommand(FleetAllocateRequest{
		Filters: []DeviceFilter{{Attribute: "platform", Operator: "EQUALS", Value: "ios"}},
		Token:   "my-token",
	})

	if response.Status != "ok" {
		t.Fatalf("expected ok, got %+v", response)
	}
	result, ok := response.Data.(FleetAllocateResponse)
	if !ok {
		t.Fatalf("expected FleetAllocateResponse, got %T", response.Data)
	}
	if result.AllocationID != "alloc-123" {
		t.Errorf("expected allocationId alloc-123, got %s", result.AllocationID)
	}
	if result.Device == nil || result.Device.ID != "R83X10DZ2TW" {
		t.Errorf("expected device R83X10DZ2TW, got %+v", result.Device)
	}
	if len(gotPaths) != 2 || gotPaths[0] != "/api/v1/sessions" || gotPaths[1] != "/api/v1/sessions/session-abc/devices" {
		t.Errorf("expected create-session then allocate-into-session, got %v", gotPaths)
	}
}

func TestFleetAllocateCommandFailsWhenSessionCreationFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "internal_error", "message": "Failed to create session"},
		})
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	response := FleetAllocateCommand(FleetAllocateRequest{Token: "my-token"})
	if response.Status != "error" {
		t.Fatalf("expected error, got %+v", response)
	}
	if !strings.Contains(response.Error, "Failed to create session") {
		t.Errorf("expected the server's error message, got %q", response.Error)
	}
}

// sessionsPage renders one page of the GET /api/v1/sessions response, embedding the given
// devices (by serial + status) into a single session with the given ID.
func sessionsPage(sessionID string, nextCursor string, devices ...map[string]string) map[string]any {
	deviceEntries := make([]map[string]any, len(devices))
	for i, d := range devices {
		deviceEntries[i] = map[string]any{
			"status": d["status"],
			"info":   map[string]string{"serial": d["serial"]},
		}
	}
	page := map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": sessionID, "devices": deviceEntries},
		},
	}
	if nextCursor != "" {
		page["nextCursor"] = nextCursor
	}
	return page
}

func TestFleetReleaseCommandFindsOwningSessionThenReleases(t *testing.T) {
	var gotPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/sessions":
			_ = json.NewEncoder(w).Encode(sessionsPage("session-abc", "", map[string]string{"serial": "R83X10DZ2TW", "status": "in_use"}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sessions/session-abc/devices/R83X10DZ2TW/release":
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	response := FleetReleaseCommand(FleetReleaseRequest{DeviceID: "R83X10DZ2TW", Token: "my-token"})
	if response.Status != "ok" {
		t.Fatalf("expected ok, got %+v", response)
	}
	if len(gotPaths) != 2 || gotPaths[1] != "POST /api/v1/sessions/session-abc/devices/R83X10DZ2TW/release" {
		t.Errorf("expected a session lookup then a release call, got %v", gotPaths)
	}
}

func TestFleetReleaseCommandPaginatesUntilTheDeviceIsFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("before") == "":
			_ = json.NewEncoder(w).Encode(sessionsPage("session-old", "2026-01-01T00:00:00Z", map[string]string{"serial": "OTHER_DEVICE", "status": "in_use"}))
		case r.Method == http.MethodGet && r.URL.Query().Get("before") == "2026-01-01T00:00:00Z":
			_ = json.NewEncoder(w).Encode(sessionsPage("session-target", "", map[string]string{"serial": "R83X10DZ2TW", "status": "in_use"}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sessions/session-target/devices/R83X10DZ2TW/release":
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			t.Errorf("unexpected request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	response := FleetReleaseCommand(FleetReleaseRequest{DeviceID: "R83X10DZ2TW", Token: "my-token"})
	if response.Status != "ok" {
		t.Fatalf("expected ok, got %+v", response)
	}
}

func TestFleetReleaseCommandFailsWhenDeviceIsNotInAnyActiveSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// same serial, but already released, so it must not match
		_ = json.NewEncoder(w).Encode(sessionsPage("session-abc", "", map[string]string{"serial": "R83X10DZ2TW", "status": "released"}))
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	response := FleetReleaseCommand(FleetReleaseRequest{DeviceID: "R83X10DZ2TW", Token: "my-token"})
	if response.Status != "error" {
		t.Fatalf("expected error, got %+v", response)
	}
	if !strings.Contains(response.Error, "R83X10DZ2TW") {
		t.Errorf("expected the error to name the device, got %q", response.Error)
	}
}
