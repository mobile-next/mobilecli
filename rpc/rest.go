package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

// RESTTimeout is the deadline for a single REST call to the fleet server.
const RESTTimeout = 30 * time.Second

// GetAPIBaseURL derives the HTTPS REST base URL from the WebSocket fleet server URL
// (same host as MOBILECLI_FLEET_URL, wss->https / ws->http, no path).
func GetAPIBaseURL() (string, error) {
	u, err := url.Parse(GetFleetServerURL())
	if err != nil {
		return "", fmt.Errorf("failed to parse fleet server URL: %w", err)
	}
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	}
	u.Path = ""
	return u.String(), nil
}

// restErrorBody mirrors the server's RESTError{Error: RESTErrorBody{Code, Message}} shape.
type restErrorBody struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// RESTCall makes an authenticated REST call to the fleet server. body is marshaled as the
// JSON request body when non-nil; result is decoded from the JSON response body when non-nil.
func RESTCall(token, method, path string, body any, result any) error {
	base, err := GetAPIBaseURL()
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal request body: %w", marshalErr)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, base+path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", utils.UserAgent())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: RESTTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call fleet server: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 300 {
		var restErr restErrorBody
		if json.Unmarshal(data, &restErr) == nil && restErr.Error.Message != "" {
			return fmt.Errorf("%s", restErr.Error.Message)
		}
		return fmt.Errorf("%s %s: unexpected status %d", method, path, resp.StatusCode)
	}

	if result != nil && len(data) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
