package wda

import "time"

func (c *WdaClient) HealthCheck() error {
	_, err := c.GetStatus()
	return err
}

func (c *WdaClient) WaitForReady(timeout time.Duration) error {
	return c.WaitForAgent() // WaitForAgent has its own 20s timeout
}

func (c *WdaClient) Close() {} // no-op — WDA lifecycle managed by testmanagerd context
