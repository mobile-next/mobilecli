package mjpeg

// func (c *DeviceKitMjpegClient) StartScreenCapture(format string, callback func([]byte) bool) error {

func (c *DeviceKitMjpegClient) CheckHealth() error {
	err := c.StartScreenCapture("mjpeg", func(data []byte) bool {
		// Just read some data and stop the stream
		return false
	})

	return err
}
