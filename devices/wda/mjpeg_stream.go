package wda

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func (c *WdaClient) SetMjpegURL(url string) {
	c.mjpegURL = url
}

func (c *WdaClient) StartMjpegStream(fps int, onData func([]byte) bool) error {
	if c.mjpegURL == "" {
		return fmt.Errorf("mjpeg URL not configured")
	}

	// configure the MJPEG framerate via WDA appium settings
	if err := c.SetMjpegFramerate(fps); err != nil {
		return fmt.Errorf("failed to set mjpeg framerate: %w", err)
	}

	client := &http.Client{
		Timeout: 0, // no timeout for long-lived streaming requests
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", c.mjpegURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to mjpeg stream: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buffer := make([]byte, 65536)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if !onData(buffer[:n]) {
				break
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading mjpeg stream: %w", err)
		}
	}

	return nil
}
