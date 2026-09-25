package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
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
	StopChan   <-chan struct{} // when non-nil, stops recording when closed (server mode; such callers also set Silent or Progress, since there is no terminal)
	Ready      chan<- error    // optional (server mode): signaled once, with nil once recording is confirmed live or with an error if it failed to start
	Silent     bool
	Progress   io.Writer // receives progress text unless Silent; defaults to os.Stderr
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
	EndReason  string `json:"endReason,omitempty"` // "size_limit" when the recording hit the size safety stop
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
	case targetDevice.Platform() == devices.PlatformAndroid:
		dev, ok := targetDevice.(*devices.AndroidDevice)
		if !ok {
			err := fmt.Errorf("expected android device")
			req.signalReady(err)
			return NewErrorResponse(err)
		}
		return screenRecordExclusive(dev.ID(), func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan)
		}, req, progress)
	case targetDevice.Platform() == devices.PlatformIOS && targetDevice.DeviceType() == devices.DeviceTypeSimulator:
		dev, ok := targetDevice.(*devices.SimulatorDevice)
		if !ok {
			err := fmt.Errorf("expected simulator device")
			req.signalReady(err)
			return NewErrorResponse(err)
		}
		return screenRecordExclusive(dev.ID(), func() error {
			return dev.ScreenRecord(req.OutputPath, req.TimeLimit, req.StopChan)
		}, req, progress)
	case targetDevice.Platform() == devices.PlatformIOS && targetDevice.DeviceType() == devices.DeviceTypeReal:
		// real iOS devices route through DeviceKit + ReplayKit; screenRecordAvc
		// signals req.Ready itself once the broadcast picker is confirmed started.
		return screenRecordAvc(targetDevice, req, progress)
	default:
		err := fmt.Errorf("screen recording is not supported for this device type")
		req.signalReady(err)
		return NewErrorResponse(err)
	}
}

// screenRecordProgress manages progress output during screen recording
type screenRecordProgress struct {
	silent      bool
	out         io.Writer
	timeLimit   int
	stopOnce    sync.Once
	tickerDone  chan struct{}
	endedCalled bool
	wasStarted  bool
}

func newScreenRecordProgress(req ScreenRecordRequest) *screenRecordProgress {
	out := req.Progress
	if out == nil {
		out = os.Stderr
	}
	return &screenRecordProgress{
		silent:     req.Silent,
		out:        out,
		timeLimit:  req.TimeLimit,
		tickerDone: make(chan struct{}),
	}
}

func (p *screenRecordProgress) started() {
	p.wasStarted = true
	if p.silent {
		return
	}
	_, _ = fmt.Fprintf(p.out, "Screen recording has started\n")
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
					_, _ = fmt.Fprintf(p.out, "\rScreen recording for %02d:%02d seconds (time limit %02d:%02d)", em, es, lm, ls)
				} else {
					_, _ = fmt.Fprintf(p.out, "\rScreen recording for %02d:%02d seconds", em, es)
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
	_, _ = fmt.Fprintf(p.out, "\nScreen recording ended, please wait while finalizing video\n")
}

func (p *screenRecordProgress) downloadProgress(downloadedMB, totalMB float64) {
	if p.silent {
		return
	}
	_, _ = fmt.Fprintf(p.out, "\rDownloading %.3f / %.3f MB", downloadedMB, totalMB)
}

func (p *screenRecordProgress) downloaded(speedMBps float64) {
	if p.silent {
		return
	}
	_, _ = fmt.Fprintf(p.out, "\nDownloading done, %.3f MB/sec\n", speedMBps)
}

// screenRecordAvc records through the device's shared H.264 stream: subscribe,
// spool the elementary stream to a temp .avc, then mux it to mp4. Used by real
// iOS devices (DeviceKit + ReplayKit), so a concurrent screencapture and
// screenrecord share one broadcast instead of stealing it from each other.
func screenRecordAvc(targetDevice devices.ControllableDevice, req ScreenRecordRequest, progress *screenRecordProgress) *CommandResponse {
	tempFile, err := os.CreateTemp("", "screenrecord-*.avc")
	if err != nil {
		req.signalReady(err)
		return NewErrorResponse(fmt.Errorf("error creating temp file: %w", err))
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()

	// in CLI mode, prevent main.go's signal handler from calling os.Exit(0)
	// before we finish converting. skip in server mode to avoid disrupting
	// the server's own signal handler.
	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	spool := newAvcSpool(tempFile, avcSizeLimit)
	err = captureAvc(targetDevice, req, progress, spool)

	progress.recordingEnded()

	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	err = errors.Join(err, spool.finish(), tempFile.Close())

	if err != nil {
		// no-op if OnReady already fired above; covers failures that happen
		// before DeviceKit/the broadcast picker was ever confirmed live.
		req.signalReady(err)
		return NewErrorResponse(fmt.Errorf("error during screen capture: %w", err))
	}

	endReason := ""
	if spool.reachedSizeLimit() {
		endReason = endReasonSizeLimit
		utils.Info("screen recording stopped at the %d byte size limit", avcSizeLimit)
	}
	return muxAvcRecording(tempPath, req, progress, endReason)
}

// captureAvc subscribes to the device's AVC stream and spools it until the
// caller stops, the time limit passes, or the spool is full.
func captureAvc(targetDevice devices.ControllableDevice, req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool) error {
	return targetDevice.StartScreenCapture(devices.ScreenCaptureConfig{
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
		OnData: withStopChan(spool.write, req.TimeLimit, req.StopChan),
		// OnData only runs when a frame arrives, and a static screen emits very
		// few; hand the stop channel down so leaving the shared stream doesn't
		// wait for the next frame.
		StopChan: req.StopChan,
	})
}

// muxAvcRecording converts the spooled stream at avcPath to the requested mp4,
// streaming from disk so memory does not grow with the recording's length.
func muxAvcRecording(avcPath string, req ScreenRecordRequest, progress *screenRecordProgress, endReason string) *CommandResponse {
	in, err := os.Open(filepath.Clean(avcPath))
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error reading temp file: %w", err))
	}
	defer func() { _ = in.Close() }()

	if info, statErr := in.Stat(); statErr == nil && info.Size() == 0 {
		// a stop that lands before the stream ever went live detaches cleanly
		// (nil error) without OnReady firing; whoever waits on Ready still
		// needs an answer. no-op if OnReady already signaled.
		err := fmt.Errorf("no data captured")
		req.signalReady(err)
		return NewErrorResponse(err)
	}

	outFile, err := os.Create(req.OutputPath)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error creating output file: %w", err))
	}

	result, err := avc2mp4.ConvertFile(in, outFile)
	err = errors.Join(err, outFile.Close())
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
		EndReason:  endReason,
	})
}

// screenRecordExclusive runs a native recorder, which serves one recording
// per device at a time; a second concurrent recording is refused.
func screenRecordExclusive(deviceID string, record func() error, req ScreenRecordRequest, progress *screenRecordProgress) *CommandResponse {
	release, err := claimNativeRecorder(deviceID)
	if err != nil {
		req.signalReady(err)
		return NewErrorResponse(err)
	}
	defer release()

	req.signalReady(nil)
	return screenRecordNative(record, req, progress)
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
