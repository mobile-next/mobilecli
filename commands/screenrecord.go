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
	OutputPath string
	TimeLimit  int              // max recording duration in seconds, 0 = no limit
	StopChan   <-chan struct{}  // when non-nil, stops recording when closed (server mode)
}

// ScreenRecordResponse contains the result of a screen recording
type ScreenRecordResponse struct {
	Output     string `json:"output"`
	FrameCount int    `json:"frameCount"`
	Duration   string `json:"duration"`
}

// ScreenRecordCommand records the device screen to an MP4 file.
func ScreenRecordCommand(req ScreenRecordRequest) *CommandResponse {
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

	// remote devices use RPC, local devices use native tools or avc capture
	if dev, ok := targetDevice.(*devices.RemoteDevice); ok {
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan)
		}, req)
	}

	switch {
	case targetDevice.Platform() == "android":
		dev, ok := targetDevice.(*devices.AndroidDevice)
		if !ok {
			return NewErrorResponse(fmt.Errorf("expected android device"))
		}
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan)
		}, req)
	case targetDevice.Platform() == "ios" && targetDevice.DeviceType() == "simulator":
		dev, ok := targetDevice.(*devices.SimulatorDevice)
		if !ok {
			return NewErrorResponse(fmt.Errorf("expected simulator device"))
		}
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan)
		}, req)
	case targetDevice.Platform() == "ios" && targetDevice.DeviceType() == "real":
		return screenRecordIOSDevice(targetDevice, req)
	default:
		return NewErrorResponse(fmt.Errorf("screen recording is not supported for this device type"))
	}
}

func screenRecordIOSDevice(targetDevice devices.ControllableDevice, req ScreenRecordRequest) *CommandResponse {
	tempFile, err := os.CreateTemp("", "screenrecord-*.avc")
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error creating temp file: %w", err))
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// in CLI mode, prevent main.go's signal handler from calling os.Exit(0)
	// before we finish converting. skip in server mode to avoid disrupting
	// the server's own signal handler.
	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	err = targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
		Format:  "avc",
		Quality: devices.DefaultQuality,
		Scale:   devices.DefaultScale,
		FPS:     devices.DefaultFramerate,
		OnProgress: func(message string) {
			utils.Verbose(message)
		},
		OnData: withStopChan(func(data []byte) bool {
			_, writeErr := tempFile.Write(data)
			return writeErr == nil
		}, req.TimeLimit, req.StopChan),
	})

	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

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
	// in CLI mode, prevent main.go's signal handler from calling os.Exit(0)
	// before the recording tool finishes. skip in server mode to avoid
	// disrupting the server's own signal handler.
	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	err := record()

	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	if err != nil {
		return NewErrorResponse(fmt.Errorf("error during screen recording: %w", err))
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", req.OutputPath)

	return NewSuccessResponse(ScreenRecordResponse{
		Output: req.OutputPath,
	})
}

// withStopChan wraps an OnData callback to stop when the time limit expires
// or the stop channel is closed. if both are zero/nil, returns the original
// callback unchanged.
func withStopChan(onData func([]byte) bool, timeLimitSec int, stopChan <-chan struct{}) func([]byte) bool {
	hasTimeLimit := timeLimitSec > 0
	hasStopChan := stopChan != nil

	if !hasTimeLimit && !hasStopChan {
		return onData
	}

	var deadline time.Time
	if hasTimeLimit {
		deadline = time.Now().Add(time.Duration(timeLimitSec) * time.Second)
	}

	return func(data []byte) bool {
		if hasTimeLimit && time.Now().After(deadline) {
			return false
		}
		if hasStopChan {
			select {
			case <-stopChan:
				return false
			default:
			}
		}
		return onData(data)
	}
}
