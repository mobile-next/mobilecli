package wda

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func (c *WdaClient) GetStatus() (map[string]any, error) {
	url := fmt.Sprintf("%s/health", c.baseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch health endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
	}

	return map[string]any{"status": "ok"}, nil
}
