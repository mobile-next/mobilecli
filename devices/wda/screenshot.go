package wda

import (
	"encoding/base64"
)

func (c *WdaClient) TakeScreenshot() ([]byte, error) {
	response, err := c.GetEndpoint("screenshot")
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
