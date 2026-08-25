package commands

import (
	"testing"
	"time"
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
