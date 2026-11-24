package commands

import (
	"encoding/json"
	"testing"
)

func decodeBootRequest(t *testing.T, params string) BootRequest {
	t.Helper()

	var req BootRequest
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		t.Fatalf("failed to decode %s: %v", params, err)
	}
	return req
}

func TestBootRequestIsNotHeadedByDefault(t *testing.T) {
	req := decodeBootRequest(t, `{"deviceId":"Pixel_9"}`)

	if req.Headed {
		t.Fatal("expected a boot without a window when headed is not given")
	}
}

func TestBootRequestReadsHeaded(t *testing.T) {
	req := decodeBootRequest(t, `{"deviceId":"Pixel_9","headed":true}`)

	if !req.Headed {
		t.Fatal("expected headed to be read from the request")
	}
}
