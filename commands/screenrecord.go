package commands

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
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
	TimeLimit  int             // max recording duration in seconds, 0 = no limit
	StopChan   <-chan struct{} // when non-nil, stops recording when closed (server mode)
	Ready      chan<- error    // optional (server mode): signaled once, with nil once recording is confirmed live or with an error if it failed to start
	Silent     bool
}

// signalReady notifies req.Ready, if present, that the recording is confirmed
// live (err == nil) or failed to start (err != nil). Safe to call more than
// once or with a nil Ready channel — only the first send has any effect.
func (req ScreenRecordRequest) signalReady(err error) {
	if req.Ready == nil {
		return
	}
	select {
	case req.Ready <- err:
	default:
	}
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
		req.signalReady(err)
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		OnProgress: func(message string) {
			utils.Verbose(message)
		},
		Hook: GetShutdownHook(),
	})
	if err != nil {
		req.signalReady(err)
		return NewErrorResponse(fmt.Errorf("error starting agent: %w", err))
	}

	progress := newScreenRecordProgress(req)

	// remote devices use RPC, local devices use native tools or avc capture
	if dev, ok := targetDevice.(*devices.RemoteDevice); ok {
		cb := &devices.ScreenRecordCallbacks{
			OnRecordingEnded:   progress.recordingEnded,
			OnDownloadProgress: progress.downloadProgress,
			OnDownloaded:       progress.downloaded,
		}
		// no async on-device UI step here (unlike real iOS devices) — the
		// recording is live as soon as we're about to dispatch it.
		req.signalReady(nil)
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan, cb)
		}, req, progress)
	}

	switch {
	case targetDevice.Platform() == "android":
		dev, ok := targetDevice.(*devices.AndroidDevice)
		if !ok {
			err := fmt.Errorf("expected android device")
			req.signalReady(err)
			return NewErrorResponse(err)
		}
		req.signalReady(nil)
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan)
		}, req, progress)
	case targetDevice.Platform() == "ios" && targetDevice.DeviceType() == "simulator":
		dev, ok := targetDevice.(*devices.SimulatorDevice)
		if !ok {
			err := fmt.Errorf("expected simulator device")
			req.signalReady(err)
			return NewErrorResponse(err)
		}
		req.signalReady(nil)
		return screenRecordNative(func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan)
		}, req, progress)
	case targetDevice.Platform() == "ios" && targetDevice.DeviceType() == "real":
		// real iOS devices route through DeviceKit + ReplayKit; screenRecordIOSDevice
		// signals req.Ready itself once the broadcast picker is confirmed started.
		return screenRecordIOSDevice(targetDevice, req, progress)
	default:
		err := fmt.Errorf("screen recording is not supported for this device type")
		req.signalReady(err)
		return NewErrorResponse(err)
	}
}

// screenRecordProgress manages progress output during screen recording
type screenRecordProgress struct {
	silent      bool
	timeLimit   int
	stopOnce    sync.Once
	tickerDone  chan struct{}
	endedCalled bool
	wasStarted  bool
}

func newScreenRecordProgress(req ScreenRecordRequest) *screenRecordProgress {
	return &screenRecordProgress{
		silent:     req.Silent || req.StopChan != nil,
		timeLimit:  req.TimeLimit,
		tickerDone: make(chan struct{}),
	}
}

func (p *screenRecordProgress) started() {
	p.wasStarted = true
	if p.silent {
		return
	}
	fmt.Fprintf(os.Stderr, "Screen recording has started\n")
}

func (p *screenRecordProgress) startTicker() {
	if p.silent {
		return
	}
	go func() {
		start := time.Now()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-p.tickerDone:
				return
			case <-ticker.C:
				elapsed := int(time.Since(start).Seconds())
				em, es := elapsed/60, elapsed%60
				if p.timeLimit > 0 {
					lm, ls := p.timeLimit/60, p.timeLimit%60
					fmt.Fprintf(os.Stderr, "\rScreen recording for %02d:%02d seconds (time limit %02d:%02d)", em, es, lm, ls)
				} else {
					fmt.Fprintf(os.Stderr, "\rScreen recording for %02d:%02d seconds", em, es)
				}
			}
		}
	}()
}

func (p *screenRecordProgress) stopTicker() {
	p.stopOnce.Do(func() { close(p.tickerDone) })
}

func (p *screenRecordProgress) recordingEnded() {
	if p.silent {
		p.endedCalled = true
		return
	}
	p.stopTicker()
	p.endedCalled = true
	// nothing to report if capture failed before it ever started
	if !p.wasStarted {
		return
	}
	fmt.Fprintf(os.Stderr, "\nScreen recording ended, please wait while finalizing video\n")
}

func (p *screenRecordProgress) downloadProgress(downloadedMB, totalMB float64) {
	if p.silent {
		return
	}
	fmt.Fprintf(os.Stderr, "\rDownloading %.3f / %.3f MB", downloadedMB, totalMB)
}

func (p *screenRecordProgress) downloaded(speedMBps float64) {
	if p.silent {
		return
	}
	fmt.Fprintf(os.Stderr, "\nDownloading done, %.3f MB/sec\n", speedMBps)
}

func screenRecordIOSDevice(targetDevice devices.ControllableDevice, req ScreenRecordRequest, progress *screenRecordProgress) *CommandResponse {
	tempFile, err := os.CreateTemp("", "screenrecord-*.avc")
	if err != nil {
		req.signalReady(err)
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
		// started/startTicker live here rather than before StartScreenCapture so
		// the displayed timer doesn't run during the ~10s DeviceKit/broadcast
		// picker setup.
		OnReady: func() {
			progress.started()
			progress.startTicker()
			req.signalReady(nil)
		},
		OnData: withStopChan(func(data []byte) bool {
			_, writeErr := tempFile.Write(data)
			return writeErr == nil
		}, req.TimeLimit, req.StopChan),
	})

	progress.recordingEnded()

	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	tempFile.Close()

	if err != nil {
		// no-op if OnReady already fired above; covers failures that happen
		// before DeviceKit/the broadcast picker was ever confirmed live.
		req.signalReady(err)
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

	if !progress.silent {
		fmt.Fprintf(os.Stderr, "%d frames, %s\nSaved video file to %s\n",
			result.FrameCount,
			result.Duration.Round(time.Millisecond),
			req.OutputPath,
		)
	}

	return NewSuccessResponse(ScreenRecordResponse{
		Output:     req.OutputPath,
		FrameCount: result.FrameCount,
		Duration:   result.Duration.Round(time.Millisecond).String(),
	})
}

func screenRecordNative(record func() error, req ScreenRecordRequest, progress *screenRecordProgress) *CommandResponse {
	// in CLI mode, prevent main.go's signal handler from calling os.Exit(0)
	// before the recording tool finishes. skip in server mode to avoid
	// disrupting the server's own signal handler.
	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	progress.started()
	progress.startTicker()

	err := record()

	progress.stopTicker()

	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	if err != nil {
		if !progress.silent {
			fmt.Fprintf(os.Stderr, "\n")
		}
		return NewErrorResponse(fmt.Errorf("error during screen recording: %w", err))
	}

	if !progress.silent {
		if !progress.endedCalled {
			// for non-remote paths, clear the ticker line
			fmt.Fprintf(os.Stderr, "\n")
		}
		fmt.Fprintf(os.Stderr, "Saved video file to %s\n", req.OutputPath)
	}

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

	// the deadline starts at the first data frame, not at wrap time: on real iOS
	// devices setup (DeviceKit + broadcast picker) can take ~10s, which would
	// otherwise expire a short time limit before any frame arrives.
	var deadline time.Time
	return func(data []byte) bool {
		if hasTimeLimit {
			if deadline.IsZero() {
				deadline = time.Now().Add(time.Duration(timeLimitSec) * time.Second)
			} else if time.Now().After(deadline) {
				return false
			}
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
