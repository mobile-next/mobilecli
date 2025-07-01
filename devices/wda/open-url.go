package wda

import "fmt"

func OpenURL(url string) error {
	sessionId, err := CreateSession()
	if err != nil {
		return err
	}

	defer DeleteSession(sessionId)

	data := map[string]interface{}{
		"url": url,
	}

	url2 := fmt.Sprintf("session/%s/url", sessionId)
	_, err = PostWebDriverAgentEndpoint(url2, data)
	if err != nil {
		return fmt.Errorf("failed to open URL: %v", err)
	}

	return nil
}
