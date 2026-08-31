package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAPIBaseURLDerivesHTTPSFromTheDefaultWSSFleetURL(t *testing.T) {
	t.Setenv("MOBILECLI_FLEET_URL", "")

	got, err := GetAPIBaseURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://api.mobilenext.ai" {
		t.Fatalf("expected https://api.mobilenext.ai, got %s", got)
	}
}

func TestGetAPIBaseURLDerivesHTTPFromAWSFleetURLOverride(t *testing.T) {
	t.Setenv("MOBILECLI_FLEET_URL", "ws://localhost:9999/ws")

	got, err := GetAPIBaseURL()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:9999" {
		t.Fatalf("expected http://localhost:9999, got %s", got)
	}
}

func TestGetAPIBaseURLRejectsAnUnparseableFleetURL(t *testing.T) {
	t.Setenv("MOBILECLI_FLEET_URL", "://missing-scheme")

	_, err := GetAPIBaseURL()
	if err == nil {
		t.Fatal("expected an error for an unparseable url")
	}
}

func TestRESTCallSendsAuthenticatedRequestAndDecodesResult(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := json.Marshal(map[string]string{"echo": "ok"})
		_ = body
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if v, ok := reqBody["filters"]; ok {
			b, _ := json.Marshal(v)
			gotBody = string(b)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "session-123"})
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	var result struct {
		ID string `json:"id"`
	}
	err := RESTCall("my-token", http.MethodPost, "/api/v1/sessions/abc/devices", map[string]any{"filters": []string{"x"}}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/api/v1/sessions/abc/devices" {
		t.Fatalf("expected /api/v1/sessions/abc/devices, got %s", gotPath)
	}
	if gotAuth != "Bearer my-token" {
		t.Fatalf("expected 'Bearer my-token', got %s", gotAuth)
	}
	if gotBody != `["x"]` {
		t.Fatalf("expected filters to be forwarded, got %s", gotBody)
	}
	if result.ID != "session-123" {
		t.Fatalf("expected result to be decoded, got %+v", result)
	}
}

func TestRESTCallReturnsServerErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"code": "insufficient_credits", "message": "account has $0.00 credits"},
		})
	}))
	defer server.Close()

	t.Setenv("MOBILECLI_FLEET_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	err := RESTCall("my-token", http.MethodPost, "/api/v1/sessions", nil, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Error() != "account has $0.00 credits" {
		t.Fatalf("expected the server's error message, got %q", err.Error())
	}
}

func TestRESTCallReportsConnectionFailure(t *testing.T) {
	t.Setenv("MOBILECLI_FLEET_URL", "ws://127.0.0.1:1/ws")

	err := RESTCall("my-token", http.MethodGet, "/api/v1/sessions", nil, nil)
	if err == nil {
		t.Fatal("expected an error when the server is unreachable")
	}
}
