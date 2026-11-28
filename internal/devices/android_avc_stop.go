package devices

import (
	"time"

	"github.com/mobile-next/mobilecli/internal/utils"
)

// A capture stopping with StopAfterAFinalFrame leaves the stream once it went
// quiet for avcFinalFrameQuiet after the requested frame began to arrive, and
// after avcFinalFrameLimit at the latest (a busy screen is never quiet).
const (
	avcFinalFrameQuiet = 100 * time.Millisecond
	avcFinalFrameLimit = 500 * time.Millisecond
)

// leaveAfterFinalFrame returns the channel that tells the hub to drop this
// capture. Once stop closes it asks for a frame, then closes the returned
// channel when chunks stopped arriving for quiet, or after limit. done ends the
// wait when the capture ended on its own; the channel is then never closed.
func leaveAfterFinalFrame(stop <-chan struct{}, requestKeyFrame func(), arrived <-chan struct{}, done <-chan struct{}, quiet, limit time.Duration) <-chan struct{} {
	leave := make(chan struct{})
	go func() {
		select {
		case <-stop:
		case <-done:
			return
		}

		deadline := time.NewTimer(limit)
		defer deadline.Stop()
		// the request is an adb round trip; it must not use up the wait
		go requestKeyFrame()
		// a chunk from before the stop is not the final frame
		select {
		case <-arrived:
		default:
		}

		var quietAfter <-chan time.Time // armed by the first chunk
		for {
			select {
			case <-arrived:
				quietAfter = time.After(quiet)
			case <-quietAfter:
				close(leave)
				return
			case <-deadline.C:
				close(leave)
				return
			case <-done:
				return
			}
		}
	}()
	return leave
}

// requestFinalFrame asks the encoder for a frame of the screen as it is now.
func (d *AndroidDevice) requestFinalFrame() {
	if err := RequestAvcKeyFrame(d); err != nil {
		utils.Verbose("could not ask for a final frame: %v", err)
	}
}
