package commands

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/pkg/avc2mp4"
	"github.com/mobile-next/mobilecli/utils"
)

// ScreenRecordRequest contains parameters for the screenrecord command
type ScreenRecordRequest struct {
	DeviceID   string
	Format     string // "mp4" or "avc"
	OutputPath string
	TimeLimit  int // max recording duration in seconds, 0 = no limit
}

// ScreenRecordResponse contains the result of a screen recording
type ScreenRecordResponse struct {
	Output     string `json:"output"`
	FrameCount int    `json:"frameCount"`
	Duration   string `json:"duration"`
}

// ScreenRecordCommand records the device screen to an MP4 file.
// for mp4 format: captures AVC stream to a temp file, converts to MP4 on stop.
// for avc format: streams raw H.264 bytes via the onData callback.
func ScreenRecordCommand(req ScreenRecordRequest, onData func([]byte) bool) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		OnProgress: func(message string) {
			utils.Verbose(message)
		},
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error starting agent: %w", err))
	}

	// for avc format, just stream to the callback
	if req.Format == "avc" {
		err = targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
			Format:  "avc",
			Quality: devices.DefaultQuality,
			Scale:   devices.DefaultScale,
			FPS:     devices.DefaultFramerate,
			OnProgress: func(message string) {
				utils.Verbose(message)
			},
			OnData: withTimeLimit(onData, req.TimeLimit),
		})
		if err != nil {
			return NewErrorResponse(fmt.Errorf("error during screen capture: %w", err))
		}
		return NewSuccessResponse(nil)
	}

	// mp4 format: android and ios simulator use native tools,
	// ios real device uses avc capture + conversion
	switch {
	case targetDevice.Platform() == "android":
		dev, ok := targetDevice.(*devices.AndroidDevice)
		if !ok {
			return NewErrorResponse(fmt.Errorf("expected android device"))
		}
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit)
		}, req)
	case targetDevice.DeviceType() == "simulator":
		dev, ok := targetDevice.(*devices.SimulatorDevice)
		if !ok {
			return NewErrorResponse(fmt.Errorf("expected simulator device"))
		}
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit)
		}, req)
	default:
		return screenRecordMp4(targetDevice, req)
	}
}

func screenRecordMp4(targetDevice devices.ControllableDevice, req ScreenRecordRequest) *CommandResponse {
	tempFile, err := os.CreateTemp("", "screenrecord-*.avc")
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error creating temp file: %w", err))
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// prevent main.go's signal handler from calling os.Exit(0) before
	// we finish converting. StartScreenCapture sets up its own handler
	// internally and will return when Ctrl+C is pressed.
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)

	err = targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
		Format:  "avc",
		Quality: devices.DefaultQuality,
		Scale:   devices.DefaultScale,
		FPS:     devices.DefaultFramerate,
		OnProgress: func(message string) {
			utils.Verbose(message)
		},
		OnData: withTimeLimit(func(data []byte) bool {
			_, writeErr := tempFile.Write(data)
			return writeErr == nil
		}, req.TimeLimit),
	})

	// restore default signal behavior so Ctrl+C during conversion
	// terminates immediately
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)

	tempFile.Close()

	if err != nil {
		return NewErrorResponse(fmt.Errorf("error during screen capture: %w", err))
	}

	// convert temp avc file to mp4
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error reading temp file: %w", err))
	}

	if len(data) == 0 {
		return NewErrorResponse(fmt.Errorf("no data captured"))
	}

	outFile, err := os.Create(req.OutputPath)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error creating output file: %w", err))
	}
	defer outFile.Close()

	result, err := avc2mp4.Convert(data, outFile)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error converting to mp4: %w", err))
	}

	fmt.Fprintf(os.Stderr, "%d frames, %s\nwrote %s\n",
		result.FrameCount,
		result.Duration.Round(time.Millisecond),
		req.OutputPath,
	)

	return NewSuccessResponse(ScreenRecordResponse{
		Output:     req.OutputPath,
		FrameCount: result.FrameCount,
		Duration:   result.Duration.Round(time.Millisecond).String(),
	})
}

func screenRecordNative(record func() error, req ScreenRecordRequest) *CommandResponse {
	// prevent main.go's signal handler from calling os.Exit(0) before
	// the recording tool finishes. ScreenRecord sets up its own handler.
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)

	err := record()

	// restore default signal behavior
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)

	if err != nil {
		return NewErrorResponse(fmt.Errorf("error during screen recording: %w", err))
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", req.OutputPath)

	return NewSuccessResponse(ScreenRecordResponse{
		Output: req.OutputPath,
	})
}

// withTimeLimit wraps an OnData callback to stop after timeLimitSec seconds.
// if timeLimitSec is 0, returns the original callback unchanged.
func withTimeLimit(onData func([]byte) bool, timeLimitSec int) func([]byte) bool {
	if timeLimitSec <= 0 {
		return onData
	}

	deadline := time.Now().Add(time.Duration(timeLimitSec) * time.Second)
	return func(data []byte) bool {
		if time.Now().After(deadline) {
			return false
		}
		return onData(data)
	}
}
