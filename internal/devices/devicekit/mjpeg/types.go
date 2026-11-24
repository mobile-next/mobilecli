package mjpeg

type DeviceKitMjpegClient struct {
	baseURL string
}

func NewDeviceKitMjpegClient(baseURL string) *DeviceKitMjpegClient {
	return &DeviceKitMjpegClient{
		baseURL: baseURL,
	}
}
