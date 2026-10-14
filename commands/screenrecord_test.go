package commands

import (
	"strings"
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
