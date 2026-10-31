package devicekit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type recordedForegroundRequest struct {
	Params map[string]any `json:"params"`
}

// agentRecordingForegroundRequests answers device.apps.foreground like an agent
// and keeps the params of the last request it received.
func agentRecordingForegroundRequests(t *testing.T, received *recordedForegroundRequest) *DeviceKitClient {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(received); err != nil {
			t.Errorf("agent could not read the request: %v", err)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"bundleId":"com.example.flutterDemo","pid":4242}}`))
	}))
	t.Cleanup(agent.Close)
	return NewDeviceKitClient(agent.URL)
}

func TestActiveAppIdentityAsksOnlyForTheBundleIDAndPid(t *testing.T) {
	var received recordedForegroundRequest
	client := agentRecordingForegroundRequests(t, &received)

	info, err := client.GetActiveAppIdentity()
	if err != nil {
		t.Fatalf("GetActiveAppIdentity: %v", err)
	}

	want := []any{"bundleId", "pid"}
	if !reflect.DeepEqual(received.Params["fields"], want) {
		t.Errorf("sent fields %v, want %v", received.Params["fields"], want)
	}
	if info.BundleID != "com.example.flutterDemo" || info.ProcessID != 4242 {
		t.Errorf("got %+v, want the bundle id and pid the agent reported", info)
	}
}

// Leaving fields out is how the agent knows to report everything, including
// the view controller that `device.apps.foreground` on the command line shows.
func TestActiveAppInfoAsksForEverything(t *testing.T) {
	var received recordedForegroundRequest
	client := agentRecordingForegroundRequests(t, &received)

	if _, err := client.GetActiveAppInfo(); err != nil {
		t.Fatalf("GetActiveAppInfo: %v", err)
	}

	if _, hasFields := received.Params["fields"]; hasFields {
		t.Errorf("sent fields %v, want none so the agent reports everything", received.Params["fields"])
	}
}

func TestActiveAppInfoReadsViewController(t *testing.T) {
	payload := []byte(`{"bundleId":"com.mobilenext.playground","name":"Playground","pid":19917,"viewController":"SwiftUI.NavigationStackHostingController<SwiftUI.AnyView>"}`)

	var info ActiveAppInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.ViewController != "SwiftUI.NavigationStackHostingController<SwiftUI.AnyView>" {
		t.Errorf("got %q, want the view controller class name", info.ViewController)
	}
}

// An agent older than the view controller support omits the field entirely.
func TestActiveAppInfoWithoutViewControllerIsEmptyNotAnError(t *testing.T) {
	payload := []byte(`{"bundleId":"com.apple.mobilesafari","name":"Safari","pid":48787}`)

	var info ActiveAppInfo
	if err := json.Unmarshal(payload, &info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.ViewController != "" {
		t.Errorf("got %q, want an empty view controller", info.ViewController)
	}

	if info.BundleID != "com.apple.mobilesafari" {
		t.Errorf("got %q, want the bundle id to still be parsed", info.BundleID)
	}
}
