package commands

// Params for the streaming CLI methods served by the daemon.

// LogsStreamRequest are the params for cli.device.logs.
type LogsStreamRequest struct {
	DeviceID string   `json:"deviceId"`
	Limit    int      `json:"limit,omitempty"`
	Filters  []string `json:"filters,omitempty"`
}

// ScreenCaptureStreamRequest are the params for cli.screencapture.
type ScreenCaptureStreamRequest struct {
	DeviceID string  `json:"deviceId"`
	Format   string  `json:"format"`
	Quality  int     `json:"quality,omitempty"`
	Scale    float64 `json:"scale,omitempty"`
	FPS      int     `json:"fps,omitempty"`
	Bitrate  int     `json:"bitrate,omitempty"`
}

// ScreenRecordStreamRequest are the params for cli.screenrecord.
type ScreenRecordStreamRequest struct {
	DeviceID   string `json:"deviceId"`
	OutputPath string `json:"output"`
	TimeLimit  int    `json:"timeLimit,omitempty"`
	Silent     bool   `json:"silent,omitempty"`
}
