package wda

func (c *WdaClient) LongPress(x, y, duration int) error {
	params := map[string]float64{
		"x":        float64(x),
		"y":        float64(y),
		"duration": float64(duration) / 1000.0, // convert ms to seconds
	}

	_, err := c.CallRPC("device.io.longpress", params)
	return err
}
