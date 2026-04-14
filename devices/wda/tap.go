package wda

func (c *WdaClient) Tap(x, y int) error {
	params := map[string]float64{
		"x": float64(x),
		"y": float64(y),
	}

	_, err := c.CallRPC("device.io.tap", params)
	return err
}
