package wda

import "fmt"

func (c *WdaClient) OpenURL(url string) error {
	sessionId, err := c.CreateSession()
	if err != nil {
		return err
	}

	defer c.DeleteSession(sessionId)

	data := map[string]interface{}{
		"url": url,
	}

	url2 := fmt.Sprintf("session/%s/url", sessionId)
	_, err = c.PostEndpoint(url2, data)
	if err != nil {
		return fmt.Errorf("failed to open URL: %v", err)
	}

	return nil
}
