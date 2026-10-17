package devices

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mobile-next/mobilecli/devices/devicekit"
)

func TestStartAgentDoesNotReuseAnotherDevicesHealthyClient(t *testing.T) {
	var requests atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer foreign.Close()
	client := devicekit.NewDeviceKitClient(foreign.URL)
	// Prove the foreign service is healthy: the old shortcut would accept it.
	if _, err := client.GetStatus(); err != nil {
		t.Fatal(err)
	}
	requests.Store(0)
	selected := &IOSDevice{Udid: "missing-device-for-agent-isolation-test", OSVersion: "16.0", deviceKitClient: client}
	err := selected.StartAgent(StartAgentConfig{})
	if err == nil {
		t.Fatal("accepted another device's healthy agent without establishing the selected device's connection")
	}
	if !strings.Contains(err.Error(), "forward") {
		t.Fatalf("expected selected-device forwarding failure, got %v", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("queried the unbound foreign client %d times", got)
	}
}
