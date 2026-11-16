package devicekit

func (c *DeviceKitClient) Swipe(x1, y1, x2, y2, duration int) error {
	params := map[string]any{
		"x1": x1,
		"y1": y1,
		"x2": x2,
		"y2": y2,
	}

	// Left out when unset so the agent keeps applying its own default.
	if duration > 0 {
		params["duration"] = float64(duration) / 1000.0 // convert ms to seconds
	}

	_, err := c.CallRPC("device.io.swipe", params)
	return err
}
