package rpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetFleetServerURLDefaultsToProduction(t *testing.T) {
	t.Setenv("MOBILECLI_FLEET_URL", "")

	if got := GetFleetServerURL(); got != defaultFleetServerURL {
		t.Fatalf("expected %s, got %s", defaultFleetServerURL, got)
	}
}

func TestGetFleetServerURLPrefersEnvironmentOverride(t *testing.T) {
	t.Setenv("MOBILECLI_FLEET_URL", "ws://localhost:9999/ws")

	if got := GetFleetServerURL(); got != "ws://localhost:9999/ws" {
		t.Fatalf("expected the override to win, got %s", got)
	}
}

// the server sends human-readable detail in Data; Message is the generic
// JSON-RPC category, so Data wins when present
func TestRPCErrorPrefersDataOverMessage(t *testing.T) {
	err := &RPCError{Code: -32000, Message: "server error", Data: "device not found"}

	if err.Error() != "device not found" {
		t.Fatalf("expected the data field, got %q", err.Error())
	}
}

func TestRPCErrorFallsBackToMessage(t *testing.T) {
	err := &RPCError{Code: -32000, Message: "server error"}

	if err.Error() != "server error" {
		t.Fatalf("expected the message field, got %q", err.Error())
	}
}

func TestRemarshalConvertsGenericMapIntoStruct(t *testing.T) {
	type device struct {
		ID   string `json:"id"`
		Port int    `json:"port"`
	}

	src := map[string]any{"id": "abc123", "port": 8080}
	var dst device

	if err := Remarshal(src, &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.ID != "abc123" || dst.Port != 8080 {
		t.Fatalf("expected {abc123 8080}, got %+v", dst)
	}
}

func TestRemarshalFailsOnUnmarshalableSource(t *testing.T) {
	var dst map[string]any

	if err := Remarshal(make(chan int), &dst); err == nil {
		t.Fatal("expected an error for a value json cannot marshal")
	}
}

func TestRemarshalFailsOnMismatchedDestination(t *testing.T) {
	var dst struct {
		Port int `json:"port"`
	}

	if err := Remarshal(map[string]any{"port": "not-a-number"}, &dst); err == nil {
		t.Fatal("expected an error when the destination type does not match")
	}
}

func TestDialRejectsAnUnparseableFleetURL(t *testing.T) {
	t.Setenv("MOBILECLI_FLEET_URL", "://missing-scheme")

	_, err := Dial("token")
	if err == nil {
		t.Fatal("expected an error for an unparseable url")
	}
	if !strings.Contains(err.Error(), "failed to parse fleet server URL") {
		t.Fatalf("expected a parse failure, got %v", err)
	}
}

func TestCallReportsConnectionFailure(t *testing.T) {
	// a server that never upgrades fails the websocket handshake deterministically,
	// instead of relying on some port happening to be closed on the host
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not a websocket endpoint", http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	err := Call("token", "devices.list", nil, nil)
	if err == nil {
		t.Fatal("expected an error when the fleet server refuses the handshake")
	}
	if !strings.Contains(err.Error(), "failed to connect to fleet server") {
		t.Fatalf("expected a connection failure, got %v", err)
	}
}
