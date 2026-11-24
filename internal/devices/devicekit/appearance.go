package devicekit

func (c *DeviceKitClient) SetAppearance(appearance string) error {
	params := map[string]string{
		"appearance": appearance,
	}

	_, err := c.CallRPC("device.settings.apply", params)
	return err
}
