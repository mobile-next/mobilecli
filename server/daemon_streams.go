package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/devices"
)

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

// StreamChunk is the notification payload for binary stream data.
type StreamChunk struct {
	Data string `json:"data"` // base64
}

// StreamMessage is the notification payload for progress text.
type StreamMessage struct {
	Message string `json:"progress"`
}

// lineNotifier turns newline-delimited JSON written to it into one
// notification per line, passing each line through untouched.
type lineNotifier struct {
	notify func(any) error
	buf    bytes.Buffer
}

func (w *lineNotifier) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadBytes('\n')
		if err != nil {
			// partial line: put it back and wait for the rest
			w.buf.Write(line)
			return len(p), nil
		}
		line = bytes.TrimRight(line, "\r\n")
		if len(line) == 0 {
			continue
		}
		if err := w.notify(json.RawMessage(line)); err != nil {
			return 0, err
		}
	}
}

// messageNotifier forwards progress text (as written, including \r) as messages.
type messageNotifier struct {
	notify func(any) error
}

func (w *messageNotifier) Write(p []byte) (int, error) {
	if err := w.notify(map[string]string{"progress": string(p)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func streamLogs(ctx context.Context, params json.RawMessage, notify func(any) error) (any, error) {
	var req LogsStreamRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return commands.NewErrorResponse(fmt.Errorf("invalid parameters: %w", err)), nil
	}
	filters, err := commands.ParseLogFilters(req.Filters)
	if err != nil {
		return commands.NewErrorResponse(err), nil
	}
	return commands.LogsCommand(ctx, commands.LogsRequest{
		DeviceID: req.DeviceID,
		Limit:    req.Limit,
		Filters:  filters,
		Writer:   &lineNotifier{notify: notify},
	}), nil
}

func streamScreenCapture(ctx context.Context, params json.RawMessage, notify func(any) error) (any, error) {
	var req ScreenCaptureStreamRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return commands.NewErrorResponse(fmt.Errorf("invalid parameters: %w", err)), nil
	}

	targetDevice, err := commands.FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return commands.NewErrorResponse(fmt.Errorf("error finding device: %v", err)), nil
	}

	onProgress := func(message string) { _ = notify(StreamMessage{Message: message}) }
	err = targetDevice.StartAgent(devices.StartAgentConfig{OnProgress: onProgress, Hook: commands.GetShutdownHook()})
	if err != nil {
		return commands.NewErrorResponse(fmt.Errorf("error starting agent: %v", err)), nil
	}

	quality := req.Quality
	if quality == 0 {
		quality = devices.DefaultQuality
	}
	scale := req.Scale
	if scale == 0 {
		scale = devices.DefaultScale
	}
	fps := req.FPS
	if fps == 0 {
		fps = devices.DefaultFramerate
	}

	err = targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
		Format:     req.Format,
		Quality:    quality,
		Scale:      scale,
		FPS:        fps,
		Bitrate:    req.Bitrate,
		OnProgress: onProgress,
		OnData: func(data []byte) bool {
			if ctx.Err() != nil {
				return false
			}
			return notify(StreamChunk{Data: base64.StdEncoding.EncodeToString(data)}) == nil
		},
	})
	if err != nil {
		return commands.NewErrorResponse(fmt.Errorf("error starting screen capture: %v", err)), nil
	}
	return commands.NewSuccessResponse(commands.OK), nil
}

func streamScreenRecord(ctx context.Context, params json.RawMessage, notify func(any) error) (any, error) {
	var req ScreenRecordStreamRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return commands.NewErrorResponse(fmt.Errorf("invalid parameters: %w", err)), nil
	}
	return commands.ScreenRecordCommand(commands.ScreenRecordRequest{
		DeviceID:   req.DeviceID,
		OutputPath: req.OutputPath,
		TimeLimit:  req.TimeLimit,
		Silent:     req.Silent,
		StopChan:   ctx.Done(), // client disconnect (Ctrl-C) stops and finalizes the recording
		Progress:   &messageNotifier{notify: notify},
	}), nil
}
