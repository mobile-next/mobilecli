package wda

func (c *WdaClient) OpenURL(url string) error {
	params := map[string]string{
		"url": url,
	}

	_, err := c.CallRPC("device.url", params)
	return err
}
