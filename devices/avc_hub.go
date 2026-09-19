package devices

import (
	"bytes"
	"errors"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mobile-next/mobilecli/pkg/avc2mp4"
	"github.com/mobile-next/mobilecli/utils"
)

// avcSource describes the single per-device H.264 source an avcHub fans out.
// start must not emit before it returns; it is expected to read in its own
// goroutine, call emit for every chunk and ended exactly once when the source
// is gone. stop makes the source end (close the conn, kill the process) and
// must not return before that teardown is complete; ended likewise must not run
// until it is. That is what lets the hub promise one live source per device.
type avcSource struct {
	start           func(emit func([]byte), ended func(error)) (stop func(), err error)
	ready           func()       // optional: this subscriber is attached and bytes are coming
	requestKeyFrame func() error // optional: ask the encoder for an immediate sync frame
}

// avcSubscriber is one consumer of the shared stream. Each has its own buffered
// channel so a slow websocket cannot stall the disk writer.
type avcSubscriber struct {
	ch      chan []byte
	joined  chan struct{} // late joiners only: closed once their first chunk is queued
	waiting bool          // late joiner, still waiting for the next IDR
	dropped atomic.Bool   // set when the hub dropped this one for falling behind
	err     error         // why this subscriber ended, set under the hub lock
}

// avcHub shares one encoder/source between every concurrent avc capture of a
// device. The first subscriber starts the source and sees every byte from the
// start; later subscribers join the running one, so their scale/fps/quality/
// bitrate are ignored. The last one leaving stops the source.
type avcHub struct {
	mu       sync.Mutex
	subs     map[*avcSubscriber]struct{}
	stop     func()
	starting chan struct{} // closed when the in-flight src.start finished
	stopping chan struct{} // closed when the in-flight stop() returned
	endErr   error         // why the last source ended
	epoch    uint64        // bumped per source, so a stopped source's leftovers are ignored
	sps      []byte        // latest parameter sets, with their start codes, for late joiners
	pps      []byte
	tail     []byte // incomplete NAL unit carried over from the previous chunk

	// set before the hub is used, so tests can run the key frame wait in
	// milliseconds; zero means the avcKeyFrame* defaults
	keyFrameTimeout time.Duration
	keyFrameRetry   time.Duration
}

const (
	nalTypeIDR = 5
	nalTypeSPS = 7
	nalTypePPS = 8
)

// avcSubscriberQueue is how many chunks a subscriber may fall behind.
// ponytail: 256 chunks (~16MB at the 64KB source reads) is the ceiling; a
// consumer that far behind is broken rather than slow, so it is dropped instead
// of being buffered forever.
const avcSubscriberQueue = 256

// avcMaxPartialNAL bounds the incomplete NAL unit carried between chunks, so a
// source that never emits another start code cannot grow the hub without limit.
const avcMaxPartialNAL = 8 << 20

// How long a late joiner waits for its first key frame, and how often the
// request is repeated meanwhile.
const (
	avcKeyFrameTimeout = 5 * time.Second
	avcKeyFrameRetry   = 1 * time.Second
)

var (
	errAvcSubscriberTooSlow = errors.New("avc stream consumer fell too far behind")
	errAvcNoKeyFrame        = errors.New("timed out waiting for a key frame on the shared avc stream")
	errAvcSourceEnded       = errors.New("avc source ended")
	errAvcCancelled         = errors.New("avc capture cancelled")
)

// watchStop reports Ctrl+C or the caller's stop channel on the returned channel,
// so one capture can leave the shared stream — promptly, even while no frames are
// arriving — without stopping the other subscribers. The returned func stops
// watching. A nil stop channel means "signals only".
func watchStop(stop <-chan struct{}) (<-chan struct{}, func()) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	cancelled := make(chan struct{})
	watching := make(chan struct{})
	go func() {
		defer signal.Stop(sigChan)
		select {
		case <-sigChan:
			close(cancelled)
		case <-stop:
			close(cancelled)
		case <-watching:
		}
	}()

	return cancelled, func() { close(watching) }
}

// subscribe attaches onData to the device's shared avc source, starting it when
// nobody else is capturing. Blocks until onData returns false, cancel is closed,
// or the source ends.
func (h *avcHub) subscribe(src avcSource, onData func([]byte) bool, cancel <-chan struct{}) error {
	sub, err := h.attach(src, cancel)
	if err != nil || sub == nil {
		return err
	}

	// a late joiner is only live once its first key frame is queued, so ready()
	// waits for that rather than firing on a stream it cannot decode yet
	if sub.joined != nil {
		if err := h.awaitFirstChunk(sub, src, cancel); err != nil {
			if errors.Is(err, errAvcCancelled) {
				return h.detach(sub, nil)
			}
			return h.detach(sub, err)
		}
	}

	if src.ready != nil {
		src.ready()
	}

	for {
		select {
		case chunk, ok := <-sub.ch:
			if !ok || sub.dropped.Load() {
				// a dropped subscriber leaves now instead of writing out a
				// queue full of stale chunks first
				return h.detach(sub, nil)
			}
			if !onData(chunk) {
				return h.detach(sub, nil)
			}
		case <-cancel:
			return h.detach(sub, nil)
		}
	}
}

// attach registers a new subscriber, starting the source when nobody else is
// capturing. It returns (nil, nil) when cancel fired while this caller was
// waiting for another subscriber's start or stop to finish.
func (h *avcHub) attach(src avcSource, cancel <-chan struct{}) (*avcSubscriber, error) {
	for {
		h.mu.Lock()
		if h.subs == nil {
			h.subs = make(map[*avcSubscriber]struct{})
		}

		// somebody else is starting or stopping the one source; wait for them
		// instead of racing a second one. on iOS a second dial would steal the
		// broadcast extension's stream.
		if busy := h.busyLocked(); busy != nil {
			h.mu.Unlock()
			if !waitOrCancel(busy, cancel) {
				return nil, nil
			}
			continue
		}

		if h.stop != nil {
			// join the running source: hand this one the cached parameter sets
			// and start its bytes at the next IDR
			sub := &avcSubscriber{ch: make(chan []byte, avcSubscriberQueue), joined: make(chan struct{}), waiting: true}
			h.subs[sub] = struct{}{}
			h.mu.Unlock()
			utils.Verbose("avc hub: joined a running stream, this request's capture settings are ignored")
			return sub, nil
		}

		started := make(chan struct{})
		h.starting = started
		h.epoch++
		epoch := h.epoch
		// registered before the start so nothing the source emits in the window
		// between src.start returning and this subscriber being installed is lost
		sub := &avcSubscriber{ch: make(chan []byte, avcSubscriberQueue)}
		h.subs[sub] = struct{}{}
		h.mu.Unlock()

		return h.startSource(src, sub, epoch, started)
	}
}

// busyLocked reports the channel to wait on while another subscriber holds the
// hub for a start or a stop, or nil when the hub is free.
func (h *avcHub) busyLocked() chan struct{} {
	if h.stopping != nil {
		return h.stopping
	}
	return h.starting
}

// startSource runs src.start outside the hub lock — on iOS it takes ~10s for
// DeviceKit and the broadcast picker — then installs the stop func. Subscribers
// waiting on the start are released either way; when it failed, one of them
// becomes the next starter and tries again.
func (h *avcHub) startSource(src avcSource, sub *avcSubscriber, epoch uint64, started chan struct{}) (*avcSubscriber, error) {
	stop, err := src.start(
		func(chunk []byte) { h.publish(epoch, chunk) },
		func(err error) { h.sourceEnded(epoch, err) },
	)

	h.mu.Lock()
	h.starting = nil
	defer close(started)

	if err != nil {
		h.finishLocked(sub, err)
		h.mu.Unlock()
		return nil, err
	}
	if stop == nil {
		stop = func() {}
	}
	if epoch != h.epoch {
		// the source already ended while it was still starting
		endErr := h.endErr
		if endErr == nil {
			endErr = errAvcSourceEnded
		}
		h.finishLocked(sub, endErr)
		h.mu.Unlock()
		stop()
		return nil, endErr
	}

	h.stop = stop
	h.mu.Unlock()
	return sub, nil
}

// awaitFirstChunk blocks until a late joiner's first chunk (parameter sets plus
// key frame) is queued. It re-asks for a key frame every avcKeyFrameRetry and
// gives up after avcKeyFrameTimeout, so a joiner is never stranded on an
// encoder that ignored the request.
func (h *avcHub) awaitFirstChunk(sub *avcSubscriber, src avcSource, cancel <-chan struct{}) error {
	giveUpAfter, retryEvery := h.keyFrameWaits()
	timeout := time.NewTimer(giveUpAfter)
	defer timeout.Stop()
	retry := time.NewTicker(retryEvery)
	defer retry.Stop()

	requestAvcKeyFrameFrom(src)
	for {
		select {
		case <-sub.joined:
			return nil
		case <-retry.C:
			requestAvcKeyFrameFrom(src)
		case <-timeout.C:
			return errAvcNoKeyFrame
		case <-cancel:
			return errAvcCancelled
		}
	}
}

// keyFrameWaits returns how long a late joiner waits for its first key frame and
// how often it re-asks for one.
func (h *avcHub) keyFrameWaits() (giveUpAfter, retryEvery time.Duration) {
	giveUpAfter, retryEvery = avcKeyFrameTimeout, avcKeyFrameRetry
	if h.keyFrameTimeout > 0 {
		giveUpAfter = h.keyFrameTimeout
	}
	if h.keyFrameRetry > 0 {
		retryEvery = h.keyFrameRetry
	}
	return giveUpAfter, retryEvery
}

// detach removes a subscriber, failing it with cause unless it already ended,
// and stops the source when it was the last one.
func (h *avcHub) detach(sub *avcSubscriber, cause error) error {
	h.mu.Lock()
	h.finishLocked(sub, cause)
	err := sub.err
	stop, stopping := h.takeStopIfIdleLocked()
	h.mu.Unlock()

	if stop != nil {
		utils.Verbose("avc hub: last subscriber left, stopping source")
		h.runStop(stop, stopping)
	}
	return err
}

// publish delivers one source chunk to every subscriber.
func (h *avcHub) publish(epoch uint64, chunk []byte) {
	h.mu.Lock()
	if epoch != h.epoch {
		// bytes from a source we already stopped
		h.mu.Unlock()
		return
	}

	buf, idrAt := h.scanLocked(chunk)
	data := buf[len(buf)-len(chunk):]
	sets := h.parameterSetsLocked()
	for sub := range h.subs {
		if !sub.waiting {
			h.sendLocked(sub, data)
			continue
		}
		// never hand a late joiner a stream it cannot decode: both parameter
		// sets must be cached and a key frame must have arrived
		if idrAt < 0 || len(h.sps) == 0 || len(h.pps) == 0 {
			continue
		}
		// most encoders repeat SPS/PPS in front of every key frame; the cached
		// copies are only for the ones that do not
		if units := avc2mp4.ParseNALUnits(buf[idrAt:]); len(units) == 0 || units[0].Type != nalTypeSPS {
			h.sendLocked(sub, sets)
		}
		h.sendLocked(sub, buf[idrAt:])
		h.markJoinedLocked(sub)
	}

	stop, stopping := h.takeStopIfIdleLocked()
	h.mu.Unlock()

	if stop != nil {
		h.runStop(stop, stopping)
	}
}

// sourceEnded fails every subscriber with the source's error and clears the hub
// so a later subscribe starts a fresh source.
func (h *avcHub) sourceEnded(epoch uint64, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if epoch != h.epoch {
		return
	}
	for sub := range h.subs {
		h.finishLocked(sub, err)
	}
	h.endErr = err
	h.clearSourceLocked()
	// a late emit from the dead source must not repopulate sps/pps/tail
	h.epoch++
}

// scanLocked splits the incoming bytes into NAL units, carrying an incomplete
// unit over to the next chunk, and caches the parameter sets. It returns that
// carried-over buffer (the previous partial NAL followed by chunk) and the
// offset of the first IDR start code in it, or -1.
func (h *avcHub) scanLocked(chunk []byte) ([]byte, int) {
	buf := make([]byte, 0, len(h.tail)+len(chunk))
	buf = append(append(buf, h.tail...), chunk...)

	// headAt is where the current picture's non-slice units (SPS, PPS, SEI)
	// began. a joiner starts there rather than at the IDR slice itself: the
	// timecode SEI that stamps a picture precedes its slice, and without it the
	// mp4 muxer has to drop the keyframe.
	idrAt, headAt := -1, -1
	pos := avc2mp4.FindStartCode(buf, 0)
	for pos >= 0 {
		next := avc2mp4.FindStartCode(buf, pos+3)
		if next < 0 {
			// the unit is still incomplete, wait for the next chunk
			break
		}
		if units := avc2mp4.ParseNALUnits(buf[pos:next]); len(units) > 0 {
			nalType := units[0].Type
			isSlice := nalType >= 1 && nalType <= nalTypeIDR
			if !isSlice && headAt < 0 {
				headAt = pos
			}
			switch nalType {
			case nalTypeSPS:
				h.sps = bytes.Clone(buf[pos:next])
			case nalTypePPS:
				h.pps = bytes.Clone(buf[pos:next])
			case nalTypeIDR:
				if idrAt < 0 {
					idrAt = pos
					if headAt >= 0 {
						idrAt = headAt
					}
				}
			}
			if isSlice {
				headAt = -1
			}
		}
		pos = next
	}

	if pos < 0 {
		// no start code yet: keep only what could be the head of one
		pos = max(len(buf)-3, 0)
	}
	if headAt >= 0 {
		// the buffer ends inside a picture's head: carry it so the slice in the
		// next chunk still finds its SEI
		pos = min(pos, headAt)
	}
	h.tail = buf[pos:len(buf):len(buf)]
	if len(h.tail) > avcMaxPartialNAL {
		utils.Verbose("avc hub: dropping %d bytes without a start code", len(h.tail))
		h.tail = nil
	}
	return buf, idrAt
}

// parameterSetsLocked returns the cached SPS and PPS a late joiner needs before
// its first keyframe.
func (h *avcHub) parameterSetsLocked() []byte {
	if len(h.sps) == 0 && len(h.pps) == 0 {
		return nil
	}
	sets := make([]byte, 0, len(h.sps)+len(h.pps))
	return append(append(sets, h.sps...), h.pps...)
}

// sendLocked queues data for one subscriber, dropping the subscriber when its
// queue is full rather than blocking the source.
func (h *avcHub) sendLocked(sub *avcSubscriber, data []byte) {
	if len(data) == 0 {
		return
	}
	if _, ok := h.subs[sub]; !ok {
		return
	}

	select {
	case sub.ch <- data:
	default:
		utils.Verbose("avc hub: subscriber fell %d chunks behind, dropping it", cap(sub.ch))
		sub.dropped.Store(true)
		h.finishLocked(sub, errAvcSubscriberTooSlow)
	}
}

// markJoinedLocked records that a late joiner's first chunk is on its way, which
// releases its awaitFirstChunk.
func (h *avcHub) markJoinedLocked(sub *avcSubscriber) {
	if !sub.waiting {
		return
	}
	sub.waiting = false
	close(sub.joined)
}

// finishLocked ends one subscriber with err; its subscribe call returns that error.
func (h *avcHub) finishLocked(sub *avcSubscriber, err error) {
	if _, ok := h.subs[sub]; !ok {
		return
	}
	delete(h.subs, sub)
	sub.err = err
	// a joiner that never got its key frame must stop waiting for one
	h.markJoinedLocked(sub)
	close(sub.ch)
}

// takeStopIfIdleLocked hands back the source's stop func once no subscriber is
// left, together with the channel reserving the hub while it runs. The caller
// must pass both to runStop after releasing the lock.
func (h *avcHub) takeStopIfIdleLocked() (func(), chan struct{}) {
	if h.stop == nil || len(h.subs) > 0 {
		return nil, nil
	}
	stop := h.stop
	h.clearSourceLocked()
	// anything the stopped source still emits belongs to the old epoch
	h.epoch++
	// hold the hub until stop() has returned, so a new subscriber cannot start a
	// second source while this one is still tearing down
	h.stopping = make(chan struct{})
	return stop, h.stopping
}

// runStop tears the source down and then releases the hub for the next subscriber.
func (h *avcHub) runStop(stop func(), stopping chan struct{}) {
	defer func() {
		h.mu.Lock()
		if h.stopping == stopping {
			h.stopping = nil
		}
		h.mu.Unlock()
		close(stopping)
	}()
	stop()
}

func (h *avcHub) clearSourceLocked() {
	h.stop = nil
	h.sps = nil
	h.pps = nil
	h.tail = nil
}

// requestAvcKeyFrameFrom asks the encoder for an immediate sync frame, if the
// source supports it.
func requestAvcKeyFrameFrom(src avcSource) {
	if src.requestKeyFrame == nil {
		return
	}
	if err := src.requestKeyFrame(); err != nil {
		utils.Verbose("avc hub: keyframe request failed: %v", err)
	}
}

// waitOrCancel blocks until done is closed, reporting false when the caller's
// cancel channel fired first.
func waitOrCancel(done <-chan struct{}, cancel <-chan struct{}) bool {
	select {
	case <-done:
		return true
	case <-cancel:
		return false
	}
}
