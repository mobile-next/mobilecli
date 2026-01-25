package commands

type AudioCaptureRequest struct {
	DeviceID string `json:"deviceId"`
	Format   string `json:"format"`
}
