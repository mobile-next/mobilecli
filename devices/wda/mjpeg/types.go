package mjpeg

type WdaMjpegClient struct {
	baseURL string
}

func NewWdaMjpegClient(baseURL string) *WdaMjpegClient {
	return &WdaMjpegClient{
		baseURL: baseURL,
	}
}
