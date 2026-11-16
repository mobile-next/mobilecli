package devicekit

func (c *DeviceKitClient) OpenURL(url string) error {
	params := map[string]string{
		"url": url,
	}

	_, err := c.CallRPC("device.url", params)
	return err
}
