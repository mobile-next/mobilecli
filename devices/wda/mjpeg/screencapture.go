package mjpeg

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

func (c *WdaMjpegClient) StartScreenCapture(format string, callback func([]byte) bool) error {

	client := &http.Client{
		Timeout: 0, // No timeout for long-lived streaming requests
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", c.baseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "*/*")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

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
