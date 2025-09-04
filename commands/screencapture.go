package commands

type ScreenCaptureRequest struct {
	DeviceID string  `json:"deviceId"`
	Format   string  `json:"format"`
	Quality  int     `json:"quality,omitempty"`
	Scale    float64 `json:"scale,omitempty"`
}
