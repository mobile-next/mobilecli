package wda

import "fmt"

func SendKeys(text string) error {

	sessionId, err := CreateSession()
	if err != nil {
		return err
	}

	defer DeleteSession(sessionId)

	url := fmt.Sprintf("session/%s/wda/keys", sessionId)
	_, err = PostWebDriverAgentEndpoint(url, map[string]interface{}{
		"value": []string{text},
	})

	return err
}
