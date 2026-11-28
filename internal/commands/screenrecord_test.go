package commands

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mobile-next/mobilecli/internal/devices"
)

func TestWithStopChanTimeLimitStartsAtFirstFrameNotAtWrapTime(t *testing.T) {
	onData := withStopChan(func([]byte) bool { return true }, 1, nil)

	// simulate slow capture setup (e.g. iOS broadcast picker) taking longer
	// than the time limit before the first frame arrives
	time.Sleep(1100 * time.Millisecond)

	if !onData(nil) {
		t.Fatal("first frame after slow setup should be accepted; time limit must start at first frame")
	}
}

func TestWithStopChanStopsAfterTimeLimitElapsedSinceFirstFrame(t *testing.T) {
	onData := withStopChan(func([]byte) bool { return true }, 1, nil)

	if !onData(nil) {
		t.Fatal("first frame should be accepted")
	}
	time.Sleep(1100 * time.Millisecond)
	if onData(nil) {
		t.Fatal("frame after time limit elapsed should stop the recording")
	}
}

func TestScreenRecordRejectsNegativeTimeLimit(t *testing.T) {
	resp := ScreenRecordCommand(ScreenRecordRequest{DeviceID: "no-such-device", TimeLimit: -1})

	if resp.Status != "error" || !strings.Contains(resp.Error, "invalid time limit -1") {
		t.Fatalf("expected an invalid time limit error, got %+v", resp)
	}
}

func TestScreenRecordRejectsTimeLimitAboveMaximum(t *testing.T) {
	if err := ValidateScreenRecordTimeLimit(MaxScreenRecordTimeLimit + 1); err == nil {
		t.Fatal("a time limit above the maximum should be rejected")
	}
}

func TestScreenRecordAcceptsNoLimitAndMaximumTimeLimit(t *testing.T) {
	for _, limit := range []int{0, 1, 180, MaxScreenRecordTimeLimit} {
		if err := ValidateScreenRecordTimeLimit(limit); err != nil {
			t.Fatalf("time limit %d should be accepted: %v", limit, err)
		}
	}
}

func TestScreenRecordReportsInvalidTimeLimitToReadyWaiter(t *testing.T) {
	ready := make(chan error, 1)
	ScreenRecordCommand(ScreenRecordRequest{TimeLimit: -5, Ready: ready})

	select {
	case err := <-ready:
		if err == nil {
			t.Fatal("ready should carry the validation error")
		}
	default:
		t.Fatal("ready was not signaled")
	}
}

func TestRealAndroidDevicesRecordFromTheSharedStream(t *testing.T) {
	if !recordsFromSharedStream(devices.PlatformAndroid, devices.DeviceTypeReal) {
		t.Fatal("a real Android device should record from its shared avc stream")
	}
}

func TestAndroidEmulatorsKeepRecordingWithScreenrecord(t *testing.T) {
	if recordsFromSharedStream(devices.PlatformAndroid, devices.DeviceTypeEmulator) {
		t.Fatal("an Android emulator should keep using adb screenrecord")
	}
}

func TestRealIOSDevicesRecordFromTheSharedStreamAndSimulatorsDoNot(t *testing.T) {
	if !recordsFromSharedStream(devices.PlatformIOS, devices.DeviceTypeReal) {
		t.Fatal("a real iOS device should record from its shared avc stream")
	}
	if recordsFromSharedStream(devices.PlatformIOS, devices.DeviceTypeSimulator) {
		t.Fatal("an iOS simulator should keep using its native recorder")
	}
}

// --- a recording whose stream ends early ----------------------------------------

// recordASecondThenLoseTheStream plays a capture that went live, recorded
// 61 pictures spanning one second, and then lost its stream (the agent died, the device
// was unplugged).
func recordASecondThenLoseTheStream(req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool) error {
	progress.started()
	req.signalReady(nil)
	stream := bytes.Join(oneSecondOfPictures(5_000_000, 61), nil)
	spoolEverything(spool, stream)
	return errors.New("the screen capture agent stopped")
}

func failBeforeGoingLive(ScreenRecordRequest, *screenRecordProgress, *avcSpool) error {
	return errors.New("failed to start the screen capture agent")
}

func silentRecordingTo(t *testing.T) ScreenRecordRequest {
	return ScreenRecordRequest{OutputPath: filepath.Join(t.TempDir(), "out.mp4"), Silent: true}
}

func TestAnAndroidRecordingWhoseStreamEndsEarlyKeepsWhatWasCaptured(t *testing.T) {
	req := silentRecordingTo(t)

	resp := recordFromAvcStream(req, newScreenRecordProgress(req), keepCapturedPart, recordASecondThenLoseTheStream)

	if resp.Status != "ok" {
		t.Fatalf("expected the captured part to be saved, got %+v", resp)
	}
	result, ok := resp.Data.(ScreenRecordResponse)
	if !ok {
		t.Fatalf("unexpected response data %T", resp.Data)
	}
	if result.EndReason != endReasonStreamEnded {
		t.Errorf("endReason = %q, want %q", result.EndReason, endReasonStreamEnded)
	}
	if result.FrameCount != 61 || result.Duration != "1s" {
		t.Errorf("got %d frames over %s, want 61 frames over 1s", result.FrameCount, result.Duration)
	}
	if info, err := os.Stat(req.OutputPath); err != nil || info.Size() == 0 {
		t.Errorf("expected a non-empty mp4 at %s: %v", req.OutputPath, err)
	}
}

func TestARecordingThatNeverWentLiveReportsTheError(t *testing.T) {
	req := silentRecordingTo(t)

	resp := recordFromAvcStream(req, newScreenRecordProgress(req), keepCapturedPart, failBeforeGoingLive)

	if resp.Status != "error" || !strings.Contains(resp.Error, "failed to start the screen capture agent") {
		t.Fatalf("expected the start error, got %+v", resp)
	}
}

func TestAnIOSRecordingWhoseStreamEndsEarlyStillFailsAsBefore(t *testing.T) {
	req := silentRecordingTo(t)

	resp := recordFromAvcStream(req, newScreenRecordProgress(req), failRecording, recordASecondThenLoseTheStream)

	if resp.Status != "error" || !strings.Contains(resp.Error, "the screen capture agent stopped") {
		t.Fatalf("expected the stream error, got %+v", resp)
	}
}

// goLiveThenLoseTheStreamBeforeAnyPicture spools only the parameter sets.
func goLiveThenLoseTheStreamBeforeAnyPicture(req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool) error {
	progress.started()
	req.signalReady(nil)
	spoolEverything(spool, bytes.Join([][]byte{sequenceParameterSet, pictureParameterSet}, nil))
	return errors.New("the screen capture agent stopped")
}

func TestASalvagedRecordingWithoutPicturesNamesWhyTheStreamEnded(t *testing.T) {
	req := silentRecordingTo(t)

	resp := recordFromAvcStream(req, newScreenRecordProgress(req), keepCapturedPart, goLiveThenLoseTheStreamBeforeAnyPicture)

	if resp.Status != "error" || !strings.Contains(resp.Error, "the screen capture agent stopped") {
		t.Fatalf("expected the stream error alongside the mux error, got %+v", resp)
	}
}

// androidShapedStream is what an Android encoder emits: SPS/PPS once, in the
// codec config buffer, then every picture behind its timecode, with a key
// frame every gopSize pictures and no parameter sets in front of them.
func androidShapedStream(firstTimestampUs uint64, pictures, gopSize int) []byte {
	units := [][]byte{sequenceParameterSet, pictureParameterSet}
	for i := range pictures {
		units = append(units, timecode(firstTimestampUs+uint64(i)*33_333))
		if i%gopSize == 0 {
			units = append(units, keyFrameSlice(byte(i)))
		} else {
			units = append(units, deltaFrameSlice(byte(i)))
		}
	}
	return bytes.Join(units, nil)
}

func TestAnAndroidShapedStreamMuxesThroughTheSpool(t *testing.T) {
	req := silentRecordingTo(t)
	spoolAndroidStream := func(req ScreenRecordRequest, progress *screenRecordProgress, spool *avcSpool) error {
		progress.started()
		for _, chunk := range inChunksOf(7, androidShapedStream(5_000_000, 91, 60)) {
			spool.write(chunk)
		}
		return nil
	}

	resp := recordFromAvcStream(req, newScreenRecordProgress(req), keepCapturedPart, spoolAndroidStream)

	result, ok := resp.Data.(ScreenRecordResponse)
	if resp.Status != "ok" || !ok {
		t.Fatalf("expected a recording, got %+v", resp)
	}
	if result.FrameCount != 91 || result.Duration != "3s" || result.EndReason != "" {
		t.Errorf("got %d frames over %s (endReason %q), want 91 frames over 3s", result.FrameCount, result.Duration, result.EndReason)
	}
}
