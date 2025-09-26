package wda

import "fmt"

func (c *WdaClient) SendKeys(text string) error {

	sessionId, err := c.CreateSession()
	if err != nil {
		return err
	}

	defer func() { _ = c.DeleteSession(sessionId) }()

	url := fmt.Sprintf("session/%s/wda/keys", sessionId)
	_, err = c.PostEndpoint(url, map[string]interface{}{
		"value": []string{text},
	})

	return err
}
