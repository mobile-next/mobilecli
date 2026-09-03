package rpc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mobile-next/mobilecli/utils"
)

func restCallAgainstServerCapturingUserAgent(t *testing.T) string {
	t.Helper()

	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("User-Agent")
		if _, err := w.Write([]byte(`{}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// GetAPIBaseURL derives the REST base from the fleet websocket URL.
	t.Setenv("MOBILECLI_FLEET_URL", strings.Replace(server.URL, "http://", "ws://", 1))

	if err := RESTCall("token", http.MethodGet, "/api/v1/sessions", nil, nil); err != nil {
		t.Fatalf("RESTCall failed: %v", err)
	}
	return seen
}

func TestRESTCallIdentifiesItselfAsMobilecli(t *testing.T) {
	got := restCallAgainstServerCapturingUserAgent(t)
	want := "mobilecli/" + utils.Version
	if got != want {
		t.Errorf("expected User-Agent %q, got %q", want, got)
	}
}
