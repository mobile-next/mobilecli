package wda

import "fmt"

func (c *WdaClient) SetAppiumSettings(settings map[string]interface{}) error {
	// create wda session
	sessionId, err := c.GetOrCreateSession()
	if err != nil {
		return fmt.Errorf("failed to create wda session: %w", err)
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
	err := c.SetAppiumSettings(map[string]interface{}{
		"mjpegServerFramerate": framerate,
	})
	if err != nil {
		return fmt.Errorf("failed to set mjpeg framerate: %w", err)
	}
	return nil
}
