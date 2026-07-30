package devicekit

func (c *DeviceKitClient) SendKeys(text string) error {
	params := map[string]string{
		"text": text,
	}

	_, err := c.CallRPC("device.io.text", params)
	return err
}
