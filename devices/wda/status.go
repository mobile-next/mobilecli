package wda

func (c *WdaClient) GetStatus() (map[string]any, error) {
	return c.GetEndpoint("status")
}
