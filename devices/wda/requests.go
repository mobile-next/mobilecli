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

func (c *WdaClient) GetEndpoint(endpoint string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch endpoint %s: %v", endpoint, err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %v", err)
	}

	return result, nil
}

func (c *WdaClient) PostEndpoint(endpoint string, data interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to post to endpoint %s: %v", endpoint, err)
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %v", err)
	}

	return result, nil
}

func (c *WdaClient) DeleteEndpoint(endpoint string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to delete endpoint %s: %v", endpoint, err)
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid JSON response: %v", err)
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
	response, err := c.PostEndpoint("session", map[string]interface{}{
		"capabilities": map[string]interface{}{
			"alwaysMatch": map[string]interface{}{
				"platformName": "iOS",
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to create session: %v", err)
	}

	// log.Printf("createSession response: %v", response)
	sessionId := response["sessionId"].(string)
	return sessionId, nil
}

func (c *WdaClient) DeleteSession(sessionId string) error {
	url := fmt.Sprintf("session/%s", sessionId)
	_, err := c.DeleteEndpoint(url)
	if err != nil {
		return fmt.Errorf("failed to delete session %s: %v", sessionId, err)
	}
	return nil
}

func (c *WdaClient) SetAppiumSettings(settings map[string]interface{}) error {
	// create wda session
	sessionId, err := c.CreateSession()
	if err != nil {
		return fmt.Errorf("failed to create wda session: %w", err)
	}
	defer c.DeleteSession(sessionId)

	// post settings to appium endpoint
	endpoint := fmt.Sprintf("session/%s/appium/settings", sessionId)
	_, err = c.PostEndpoint(endpoint, map[string]interface{}{
		"settings": settings,
	})
	if err != nil {
		return fmt.Errorf("failed to set appium settings: %w", err)
	}

	return nil
}
