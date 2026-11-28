package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mobile-next/mobilecli/internal/devices"
)

// recordingConfigFor builds the capture config a recording on platform hands
// the device, spooling into a temp file.
func recordingConfigFor(t *testing.T, platform string, req ScreenRecordRequest) (devices.ScreenCaptureConfig, func()) {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "spool.avc"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	req.Silent = true
	return avcRecordingConfig(platform, req, newScreenRecordProgress(req), newAvcSpool(file, avcSizeLimit), time.Second)
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func waitForClose(t *testing.T, ch <-chan struct{}, within time.Duration) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(within):
		t.Fatal("the recording was not told to stop")
	}
}

func TestAnAndroidTimeLimitStopsTheRecordingOnAStaticScreen(t *testing.T) {
	config, release := recordingConfigFor(t, devices.PlatformAndroid, ScreenRecordRequest{TimeLimit: 1})
	defer release()

	config.OnData(sequenceParameterSet) // the first chunk starts the clock; then the screen is static

	waitForClose(t, config.StopChan, 2*time.Second)
	if config.StopMode != devices.StopAfterAFinalFrame {
		t.Fatal("a recording ended by its time limit must still take a final frame")
	}
}

func TestAnAndroidTimeLimitStillSpoolsTheFinalFrame(t *testing.T) {
	config, release := recordingConfigFor(t, devices.PlatformAndroid, ScreenRecordRequest{TimeLimit: 1})
	defer release()
	config.OnData(sequenceParameterSet)
	waitForClose(t, config.StopChan, 2*time.Second)

	if !config.OnData(pictureParameterSet) {
		t.Fatal("the frame asked for at the stop must be spooled, not refused")
	}
}

func TestAnAndroidTimeLimitStartsAtTheFirstChunk(t *testing.T) {
	config, release := recordingConfigFor(t, devices.PlatformAndroid, ScreenRecordRequest{TimeLimit: 1})
	defer release()

	time.Sleep(1200 * time.Millisecond) // setup before any video

	if isClosed(config.StopChan) {
		t.Fatal("the time limit must not run before the first chunk")
	}
}

func TestAnAndroidRecordingStoppedByTheCallerStopsAtOnce(t *testing.T) {
	stop := make(chan struct{})
	config, release := recordingConfigFor(t, devices.PlatformAndroid, ScreenRecordRequest{TimeLimit: 60, StopChan: stop})
	defer release()

	close(stop)

	waitForClose(t, config.StopChan, time.Second)
}

func TestAnIOSTimeLimitStillEndsInOnData(t *testing.T) {
	config, release := recordingConfigFor(t, devices.PlatformIOS, ScreenRecordRequest{TimeLimit: 1})
	defer release()

	config.OnData(sequenceParameterSet)
	time.Sleep(1100 * time.Millisecond)

	if config.OnData(pictureParameterSet) {
		t.Fatal("iOS keeps its time limit in OnData")
	}
	if config.StopMode != devices.StopRightAway {
		t.Fatal("iOS has no final frame")
	}
}
