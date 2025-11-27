package wda

import "fmt"

func (c *WdaClient) SetAppiumSettings(settings map[string]interface{}) error {
	// get or create wda session
	sessionId, err := c.GetOrCreateSession()
	if err != nil {
		return fmt.Errorf("failed to get or create wda session: %w", err)
	}

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

func (c *WdaClient) SetMjpegFramerate(framerate int) error {
	var err error
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		err = c.SetAppiumSettings(map[string]interface{}{
			"mjpegServerFramerate": framerate,
		})

		if err == nil {
			return nil
		}

		fmt.Printf("attempt %d/%d failed to set mjpeg framerate: %v\n", i+1, maxRetries, err)
	}

	return fmt.Errorf("failed to set mjpeg framerate after %d attempts: %w", maxRetries, err)
}
