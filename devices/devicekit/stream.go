package devicekit

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) StartH264Stream(fps, quality int, scale float64, onData func([]byte) bool) error {
	url := fmt.Sprintf("%s/h264?fps=%d", c.httpURL, fps)
	if quality > 0 {
		url += fmt.Sprintf("&quality=%d", quality)
	}
	if scale > 0 {
		url += fmt.Sprintf("&scale=%d", int(scale*100))
	}

	client := &http.Client{
		Timeout: 0, // no timeout for long-lived streaming requests
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to h264 stream: %w", err)
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
			return fmt.Errorf("error reading h264 stream: %w", err)
		}
	}

	return nil
}

func (c *Client) StartMjpegStream(fps int, onData func([]byte) bool) error {
	url := fmt.Sprintf("%s/mjpeg?fps=%d", c.httpURL, fps)

	client := &http.Client{
		Timeout: 0, // no timeout for long-lived streaming requests
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
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
