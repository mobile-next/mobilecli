package devices

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// finalFrameWait runs leaveAfterFinalFrame with short waits and reports what
// the stop did.
type finalFrameWait struct {
	stop      chan struct{}
	arrived   chan struct{}
	done      chan struct{}
	requests  atomic.Int32
	leave     <-chan struct{}
	startedAt time.Time
}

func newFinalFrameWait(quiet, limit time.Duration) *finalFrameWait {
	w := &finalFrameWait{
		stop:    make(chan struct{}),
		arrived: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	w.leave = leaveAfterFinalFrame(w.stop, func() { w.requests.Add(1) }, w.arrived, w.done, quiet, limit)
	return w
}

func (w *finalFrameWait) stopTheRecording() {
	w.startedAt = time.Now()
	close(w.stop)
}

func (w *finalFrameWait) chunkArrives() { signalNonBlocking(w.arrived) }

// leftAfter waits for the hub to be told to leave and says how long that took.
func (w *finalFrameWait) leftAfter(t *testing.T) time.Duration {
	t.Helper()
	select {
	case <-w.leave:
		return time.Since(w.startedAt)
	case <-time.After(2 * time.Second):
		t.Fatal("the capture never left the stream")
		return 0
	}
}

func TestAStoppedRecordingAsksForAFinalFrameAndLeavesOnceItArrived(t *testing.T) {
	w := newFinalFrameWait(20*time.Millisecond, time.Second)

	w.stopTheRecording()
	time.Sleep(10 * time.Millisecond)
	w.chunkArrives()

	assert.Less(t, w.leftAfter(t), 500*time.Millisecond, "should leave once the stream went quiet, not at the limit")
	assert.Equal(t, int32(1), w.requests.Load())
}

func TestAStoppedRecordingLeavesAtTheLimitWhenNoFrameComes(t *testing.T) {
	w := newFinalFrameWait(20*time.Millisecond, 100*time.Millisecond)

	w.stopTheRecording()

	assert.GreaterOrEqual(t, w.leftAfter(t), 100*time.Millisecond)
}

func TestAStoppedRecordingOnABusyScreenLeavesAtTheLimit(t *testing.T) {
	w := newFinalFrameWait(50*time.Millisecond, 150*time.Millisecond)
	busy := make(chan struct{})
	defer close(busy)
	go func() {
		for {
			select {
			case <-busy:
				return
			case <-time.After(5 * time.Millisecond):
				w.chunkArrives()
			}
		}
	}()

	w.stopTheRecording()

	left := w.leftAfter(t)
	assert.GreaterOrEqual(t, left, 150*time.Millisecond)
	assert.Less(t, left, time.Second)
}

func TestTheFinalFrameWaitEndsWithTheCapture(t *testing.T) {
	w := newFinalFrameWait(20*time.Millisecond, time.Second)

	close(w.done)

	select {
	case <-w.leave:
		t.Fatal("a capture that ended on its own must not be told to leave")
	case <-time.After(50 * time.Millisecond):
	}
	assert.Equal(t, int32(0), w.requests.Load())
}
