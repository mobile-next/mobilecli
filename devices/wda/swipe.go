package wda

func (c *WdaClient) Swipe(x1, y1, x2, y2 int) error {
	params := map[string]int{
		"x1": x1,
		"y1": y1,
		"x2": x2,
		"y2": y2,
	}

	_, err := c.CallRPC("device.io.swipe", params)
	return err
}
