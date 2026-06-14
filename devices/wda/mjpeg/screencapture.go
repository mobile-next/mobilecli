package mjpeg

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (c *WdaMjpegClient) StartScreenCapture(format string, callback func([]byte) bool) error {

	client := &http.Client{
		Timeout: 0, // no timeout for long-lived streaming requests
	}

	// the MJPEG server on the device may not be ready immediately after the
	// WDA HTTP endpoint comes up, so retry the connection for a few seconds
	var resp *http.Response
	deadline := time.Now().Add(10 * time.Second)
	for {
		req, err := http.NewRequestWithContext(context.Background(), "GET", c.baseURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "*/*")

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}

		if resp != nil {
			_ = resp.Body.Close()
		}

		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("failed to connect to MJPEG stream: %w", err)
			}
			return fmt.Errorf("MJPEG stream returned status %d", resp.StatusCode)
		}
		time.Sleep(500 * time.Millisecond)
	}

	defer func() { _ = resp.Body.Close() }()

	buffer := make([]byte, 65536)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if !callback(buffer[:n]) {
				// client wants to stop the stream
				fmt.Println("Screen capture ended by client")
				break
			}
		}

		if err != nil {
			if err == io.EOF {
				// Normal end of stream
				break
			}

			return fmt.Errorf("error reading response body: %w", err)
		}
	}

	fmt.Println("Screen capture ended")
	return nil
}
