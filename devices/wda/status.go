package wda

func (c *WdaClient) GetStatus() (map[string]interface{}, error) {
	return c.GetEndpoint("status")
}
