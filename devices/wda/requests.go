package wda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

type alwaysMatch struct {
	PlatformName string `json:"platformName"`
}

type sessionCapabilities struct {
	AlwaysMatch alwaysMatch `json:"alwaysMatch"`
}

type sessionRequest struct {
	Capabilities sessionCapabilities `json:"capabilities"`
}

func (c *WdaClient) getEndpointWithTimeout(endpoint string, timeout time.Duration) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch endpoint %s: %w", endpoint, err)
	}

	defer func() { _ = resp.Body.Close() }()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("endpoint %s returned non-2xx status: %d, body: %s", endpoint, resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	return result, nil
}

func (c *WdaClient) GetEndpoint(endpoint string) (map[string]interface{}, error) {
	return c.getEndpointWithTimeout(endpoint, 5*time.Second)
}

func (c *WdaClient) PostEndpoint(endpoint string, data interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to post to endpoint %s: %w", endpoint, err)
	}

	defer func() { _ = resp.Body.Close() }()

	// read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("endpoint %s returned non-2xx status: %d, body: %s", endpoint, resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	return result, nil
}

func (c *WdaClient) DeleteEndpoint(endpoint string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to delete endpoint %s: %w", endpoint, err)
	}

	defer func() { _ = resp.Body.Close() }()

	// read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// check HTTP status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("endpoint %s returned non-2xx status: %d, body: %s", endpoint, resp.StatusCode, string(body))
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %w", err)
	}

	return result, nil
}

// waitForWebDriverAgent waits for WebDriverAgent to be ready by polling its status endpoint
func (c *WdaClient) WaitForAgent() error {
	// Set timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for WebDriverAgent to be ready")

		case <-ticker.C:
			_, err := c.GetStatus()
			if err != nil {
				utils.Verbose("WebDriverAgent not ready yet: %v", err)
				continue
			}

			utils.Verbose("WebDriverAgent is ready!")
			return nil
		}
	}
}

func (c *WdaClient) CreateSession() (string, error) {
	request := sessionRequest{
		Capabilities: sessionCapabilities{
			AlwaysMatch: alwaysMatch{
				PlatformName: "iOS",
			},
		},
	}

	response, err := c.PostEndpoint("session", request)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	// log.Printf("createSession response: %v", response)
	sessionId := response["sessionId"].(string)
	return sessionId, nil
}

// isSessionStillValid checks if a session is still valid
func (c *WdaClient) isSessionStillValid(sessionId string) bool {
	endpoint := fmt.Sprintf("session/%s", sessionId)
	_, err := c.GetEndpoint(endpoint)
	return err == nil
}

// GetOrCreateSession returns cached session or creates a new one
func (c *WdaClient) GetOrCreateSession() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// if we have a cached session, validate it first
	if c.sessionId != "" {
		if c.isSessionStillValid(c.sessionId) {
			return c.sessionId, nil
		}

		// session is invalid, clear it and create a new one
		utils.Verbose("cached session %s is invalid, creating new session", c.sessionId)
		c.sessionId = ""
	}

	sessionId, err := c.CreateSession()
	if err != nil {
		return "", err
	}

	c.sessionId = sessionId
	return sessionId, nil
}

func (c *WdaClient) DeleteSession(sessionId string) error {
	url := fmt.Sprintf("session/%s", sessionId)
	_, err := c.DeleteEndpoint(url)
	if err != nil {
		return fmt.Errorf("failed to delete session %s: %w", sessionId, err)
	}

	c.mu.Lock()
	if c.sessionId == sessionId {
		c.sessionId = ""
	}
	c.mu.Unlock()

	return nil
}
