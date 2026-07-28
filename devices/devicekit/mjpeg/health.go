package mjpeg

// func (c *WdaMjpegClient) StartScreenCapture(format string, callback func([]byte) bool) error {

func (c *WdaMjpegClient) CheckHealth() error {
	err := c.StartScreenCapture("mjpeg", func(data []byte) bool {
		// Just read some data and stop the stream
		return false
	})

	return err
}
