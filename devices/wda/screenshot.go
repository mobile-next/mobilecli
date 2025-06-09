package wda

import (
	"encoding/base64"
)

func TakeScreenshot() ([]byte, error) {
	response, err := GetWebDriverAgentEndpoint("screenshot")
	if err != nil {
		return nil, err
	}

	screenshotData := response["value"].(string)
	screenshotBytes, err := base64.StdEncoding.DecodeString(screenshotData)
	if err != nil {
		return nil, err
	}

	return screenshotBytes, nil
}
