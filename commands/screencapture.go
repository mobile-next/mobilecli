package commands

type ScreenCaptureRequest struct {
	DeviceID string `json:"deviceId"`
	Format   string `json:"format"`
}
