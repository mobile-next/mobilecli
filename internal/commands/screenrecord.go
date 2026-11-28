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

	"github.com/mobile-next/mobilecli/internal/avc2mp4"
	"github.com/mobile-next/mobilecli/internal/devices"
	"github.com/mobile-next/mobilecli/internal/utils"
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
	EndReason  string `json:"endReason,omitempty"` // "size_limit" when the recording hit the size safety stop, "stream_ended" when the device stream stopped first
}

// MaxScreenRecordTimeLimit is the longest time limit, in seconds, a recording
// accepts: the largest value Android's screenrecord takes (older Android
// versions refuse anything above 180, and that error is reported as is).
const MaxScreenRecordTimeLimit = 4294967295

// ValidateScreenRecordTimeLimit rejects a time limit that is negative or
// above MaxScreenRecordTimeLimit. 0 means no limit.
func ValidateScreenRecordTimeLimit(timeLimit int) error {
	if timeLimit < 0 || int64(timeLimit) > MaxScreenRecordTimeLimit {
		return fmt.Errorf("invalid time limit %d: must be 0 (no limit) or between 1 and %d seconds", timeLimit, MaxScreenRecordTimeLimit)
	}
	return nil
}

// ResolveScreenRecordPath turns the requested output into an absolute file path.
// A path ending in a separator (or an empty path, meaning the current
// directory) gets a generated screenrecord-<device>-<timestamp>.mp4 filename.
// A generated name stays reserved until the returned release is called, so a
// second recording that starts before this one is written gets another name.
func ResolveScreenRecordPath(outputPath, deviceID string) (string, func(), error) {
	release := func() {}
	path, err := resolveOutputPath(outputPath, "screenrecord", deviceID, "mp4", func(generated string) string {
		var reserved string
		reserved, release = reserveRecordingPath(generated)
		return reserved
	})
	return path, release, err
}

// ScreenRecordCommand records the device screen to an MP4 file.
func ScreenRecordCommand(req ScreenRecordRequest) *CommandResponse {
	if err := ValidateScreenRecordTimeLimit(req.TimeLimit); err != nil {
		req.signalReady(err)
		return NewErrorResponse(err)
	}

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
	case recordsFromSharedStream(targetDevice.Platform(), targetDevice.DeviceType()):
		// screenRecordAvc signals req.Ready itself once the stream is live (on
		// real iOS devices, once the broadcast picker is confirmed started).
		return screenRecordAvc(targetDevice, req, progress)
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
	default:
		err := fmt.Errorf("screen recording is not supported for this device type")
		req.signalReady(err)
		return NewErrorResponse(err)
	}
}

// recordsFromSharedStream tells whether a device records by subscribing to its
// one shared H.264 stream, the one live captures use too, rather than with a
// native recorder. Real devices do: no time cap, one encoder for the stream
// and the recording. Android emulators keep adb screenrecord and iOS
// simulators their native recorder.
func recordsFromSharedStream(platform, deviceType string) bool {
	if deviceType != devices.DeviceTypeReal {
		return false
	}
	return platform == devices.PlatformAndroid || platform == devices.PlatformIOS
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
// iOS and Android devices, so a concurrent screencapture and screenrecord share
// one encoder instead of stealing it from each other.
func screenRecordAvc(targetDevice devices.ControllableDevice, req ScreenRecordRequest, progress *screenRecordProgress) *CommandResponse {
	// iOS keeps its behaviour: its stream ends cleanly on EOF, and a read
	// error fails the recording as it always did
	policy := failRecording
	if targetDevice.Platform() == devices.PlatformAndroid {
		policy = keepCapturedPart
	}
	return recordFromAvcStream(req, progress, policy, func(req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool) error {
		return captureAvc(targetDevice, req, progress, spool)
	})
}

// avcCapture spools a device's avc stream until the recording stops.
type avcCapture func(req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool) error

// streamEndPolicy is what a recording does when its stream ends with an error
// after the recording went live.
type streamEndPolicy int

const (
	// failRecording reports the error and saves nothing
	failRecording streamEndPolicy = iota
	// keepCapturedPart saves what was captured as a complete mp4, with
	// endReason "stream_ended"
	keepCapturedPart
)

// recordFromAvcStream runs capture into a temp spool and muxes the result; see
// streamEndPolicy for a stream that ends early.
func recordFromAvcStream(req ScreenRecordRequest, progress *screenRecordProgress, policy streamEndPolicy, capture avcCapture) *CommandResponse {
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
	captureErr := capture(req, progress, spool)

	progress.recordingEnded()

	if req.StopChan == nil {
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}

	err = errors.Join(spool.finish(), tempFile.Close())
	keepsPart := policy == keepCapturedPart && captureErr != nil && progress.wasStarted && spool.hasData()
	if !keepsPart {
		err = errors.Join(captureErr, err)
	}

	if err != nil {
		// no-op if OnReady already fired above; covers failures that happen
		// before the stream (on iOS, the broadcast picker) was confirmed live.
		req.signalReady(err)
		return NewErrorResponse(fmt.Errorf("error during screen capture: %w", err))
	}

	endReason := ""
	if keepsPart {
		endReason = endReasonStreamEnded
		utils.Info("screen recording ended early, saving what was captured: %v", captureErr)
	}
	if spool.reachedSizeLimit() {
		endReason = endReasonSizeLimit
		utils.Info("screen recording stopped at the %d byte size limit", avcSizeLimit)
	}
	var streamErr error
	if keepsPart {
		streamErr = captureErr
	}
	return muxAvcRecording(tempPath, req, progress, endReason, streamErr)
}

// captureAvc subscribes to the device's AVC stream and spools it until the
// caller stops, the time limit passes, or the spool is full.
func captureAvc(targetDevice devices.ControllableDevice, req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool) error {
	config, release := avcRecordingConfig(targetDevice.Platform(), req, progress, spool, time.Second)
	defer release()
	return targetDevice.StartScreenCapture(config)
}

// avcRecordingConfig is the capture config of a recording on platform; the
// returned release must be called once the capture ended. timeLimitUnit is
// what one unit of req.TimeLimit lasts (a second; tests shorten it).
//
// How a recording ends:
//   - the caller stops it (StopChan, which is the request's context in the
//     daemon) or the time limit passes: on Android both close the capture's
//     StopChan, so the device takes one final frame before leaving and a
//     static screen at the end stays in the video. The time limit is a timer,
//     not a check on the next chunk, which a static screen may never send.
//   - the spool is full (size cap): OnData refuses the chunk and the capture
//     leaves at once; there is no room for a final frame.
//   - the stream dies: the capture ends with its error; nothing more comes.
//
// iOS keeps its earlier behaviour: the time limit and the stop are checked in
// OnData and the capture leaves at once.
func avcRecordingConfig(platform string, req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool, timeLimitUnit time.Duration) (devices.ScreenCaptureConfig, func()) {
	config := devices.ScreenCaptureConfig{
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
		StopMode: devices.StopRightAway,
	}
	if platform != devices.PlatformAndroid {
		return config, func() {}
	}

	limit := time.Duration(req.TimeLimit) * timeLimitUnit
	stop, firstChunk, release := stopAfterTimeLimit(req.StopChan, limit)
	config.OnData = func(chunk []byte) bool {
		firstChunk()
		return spool.write(chunk)
	}
	config.StopChan = stop
	config.StopMode = devices.StopAfterAFinalFrame
	return config, release
}

// stopAfterTimeLimit returns a channel that closes when callerStop closes, or
// limit after firstChunk was first called (never when limit is 0). release
// ends the wait.
func stopAfterTimeLimit(callerStop <-chan struct{}, limit time.Duration) (stop <-chan struct{}, firstChunk func(), release func()) {
	stopped := make(chan struct{})
	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		// the limit starts at the first chunk, not now: setup (on iOS the
		// broadcast picker) can take longer than a short limit
		var waitStart <-chan struct{}
		if limit > 0 {
			waitStart = started
		}
		var expired <-chan time.Time
		for {
			select {
			case <-callerStop:
				close(stopped)
				return
			case <-waitStart:
				waitStart = nil
				timer := time.NewTimer(limit)
				defer timer.Stop()
				expired = timer.C
			case <-expired:
				utils.Verbose("screen recording reached its %s time limit", limit)
				close(stopped)
				return
			case <-done:
				return
			}
		}
	}()

	return stopped, sync.OnceFunc(func() { close(started) }), sync.OnceFunc(func() { close(done) })
}

// muxAvcRecording converts the spooled stream at avcPath to the requested mp4,
// streaming from disk so memory does not grow with the recording's length.
// streamErr is why the stream ended early, if it did; it is part of any error.
func muxAvcRecording(avcPath string, req ScreenRecordRequest, progress *screenRecordProgress, endReason string, streamErr error) *CommandResponse {
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
	if err != nil && streamErr != nil {
		err = fmt.Errorf("%w (the stream ended early: %w)", err, streamErr)
	}
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error converting to mp4: %w", err))
	}

	if !progress.silent {
		_, _ = fmt.Fprintf(progress.out, "%d frames, %s\nSaved video file to %s\n",
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
			_, _ = fmt.Fprintf(progress.out, "\n")
		}
		return NewErrorResponse(fmt.Errorf("error during screen recording: %w", err))
	}

	if !progress.silent {
		if !progress.endedCalled {
			// for non-remote paths, clear the ticker line
			_, _ = fmt.Fprintf(progress.out, "\n")
		}
		_, _ = fmt.Fprintf(progress.out, "Saved video file to %s\n", req.OutputPath)
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
