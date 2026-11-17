package devices

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- annex-b builders -------------------------------------------------------

// nal builds one annex-b NAL unit: a 4-byte start code, the type byte, payload.
func nal(nalType byte, payload ...byte) []byte {
	return append([]byte{0x00, 0x00, 0x00, 0x01, nalType}, payload...)
}

func spsNAL() []byte                   { return nal(nalTypeSPS, 0xaa) }
func ppsNAL() []byte                   { return nal(nalTypePPS, 0xbb) }
func keyFrameNAL(marker byte) []byte   { return nal(nalTypeIDR, marker) }
func deltaFrameNAL(marker byte) []byte { return nal(1, marker) }
func timecodeNAL(marker byte) []byte   { return nal(6, marker) }
func concat(parts ...[]byte) []byte    { return bytes.Join(parts, nil) }
func splitIntoSingleBytes(b []byte) [][]byte {
	chunks := make([][]byte, 0, len(b))
	for i := range b {
		chunks = append(chunks, b[i:i+1])
	}
	return chunks
}

// --- fake source ------------------------------------------------------------

// fakeAvcSource stands in for a device encoder: the test feeds it bytes and
// observes when the hub starts it, stops it, or asks for a key frame.
type fakeAvcSource struct {
	mu           sync.Mutex
	starts       int
	keyFrameReqs int
	live         int // sources currently between start and a completed stop
	maxLive      int
	emit         func([]byte)
	ended        func(error)
	keyFrameErr  error
	startGate    chan struct{} // when set, start blocks on it
	stopGate     chan struct{} // when set, stop blocks on it
	startBegan   chan struct{}
	stopBegan    chan struct{}
	started      chan struct{}
	stopped      chan struct{}
	keyFrames    chan struct{}
}

func newFakeAvcSource() *fakeAvcSource {
	return &fakeAvcSource{
		startBegan: make(chan struct{}, 16),
		stopBegan:  make(chan struct{}, 16),
		started:    make(chan struct{}, 16),
		stopped:    make(chan struct{}, 16),
		keyFrames:  make(chan struct{}, 16),
	}
}

func (f *fakeAvcSource) source() avcSource {
	return avcSource{
		start:           f.start,
		requestKeyFrame: f.requestKeyFrame,
	}
}

func (f *fakeAvcSource) start(emit func([]byte), ended func(error)) (func(), error) {
	f.mu.Lock()
	gate := f.startGate
	f.mu.Unlock()

	signalNonBlocking(f.startBegan)
	if gate != nil {
		<-gate
	}

	f.mu.Lock()
	f.starts++
	f.live++
	if f.live > f.maxLive {
		f.maxLive = f.live
	}
	f.emit, f.ended = emit, ended
	stopGate := f.stopGate
	f.mu.Unlock()
	signalNonBlocking(f.started)

	return func() {
		signalNonBlocking(f.stopBegan)
		if stopGate != nil {
			<-stopGate
		}
		f.mu.Lock()
		f.live--
		f.mu.Unlock()
		signalNonBlocking(f.stopped)
	}, nil
}

func (f *fakeAvcSource) requestKeyFrame() error {
	f.mu.Lock()
	f.keyFrameReqs++
	err := f.keyFrameErr
	f.mu.Unlock()
	signalNonBlocking(f.keyFrames)
	return err
}

// blockStart makes the next start hang until the returned func is called, the
// way DeviceKit plus the broadcast picker hangs for ~10s on a real iOS device.
func (f *fakeAvcSource) blockStart() func() {
	gate := make(chan struct{})
	f.mu.Lock()
	f.startGate = gate
	f.mu.Unlock()
	return func() { close(gate) }
}

// blockStop makes stop hang until the returned func is called, standing in for a
// source that is still tearing down.
func (f *fakeAvcSource) blockStop() func() {
	gate := make(chan struct{})
	f.mu.Lock()
	f.stopGate = gate
	f.mu.Unlock()
	return func() { close(gate) }
}

// neverProducesKeyFrames makes every key frame request fail, like an encoder
// that ignores the request.
func (f *fakeAvcSource) neverProducesKeyFrames(err error) {
	f.mu.Lock()
	f.keyFrameErr = err
	f.mu.Unlock()
}

func (f *fakeAvcSource) keyFrameRequestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.keyFrameReqs
}

func (f *fakeAvcSource) mostSourcesAliveAtOnce() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxLive
}

func signalNonBlocking(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (f *fakeAvcSource) feed(chunks ...[]byte) {
	f.mu.Lock()
	emit := f.emit
	f.mu.Unlock()
	for _, chunk := range chunks {
		emit(chunk)
	}
}

func (f *fakeAvcSource) die(err error) {
	f.mu.Lock()
	ended := f.ended
	f.mu.Unlock()
	ended(err)
}

func (f *fakeAvcSource) startCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts
}

func (f *fakeAvcSource) waitUntilStarted(t *testing.T) { waitForSignal(t, f.started, "source start") }
func (f *fakeAvcSource) waitUntilStopped(t *testing.T) { waitForSignal(t, f.stopped, "source stop") }
func (f *fakeAvcSource) waitUntilStartBegan(t *testing.T) {
	waitForSignal(t, f.startBegan, "the source start to begin")
}
func (f *fakeAvcSource) waitUntilStopBegan(t *testing.T) {
	waitForSignal(t, f.stopBegan, "the source stop to begin")
}

func (f *fakeAvcSource) expectNotStartedAgain(t *testing.T) {
	t.Helper()
	select {
	case <-f.startBegan:
		t.Fatal("a second source was started while the first one was still stopping")
	case <-time.After(50 * time.Millisecond):
	}
}
func (f *fakeAvcSource) waitForKeyFrameRequest(t *testing.T) {
	waitForSignal(t, f.keyFrames, "key frame request")
}

func (f *fakeAvcSource) expectStillRunning(t *testing.T) {
	t.Helper()
	select {
	case <-f.stopped:
		t.Fatal("source was stopped while subscribers were still attached")
	case <-time.After(50 * time.Millisecond):
	}
}

// --- fake subscriber --------------------------------------------------------

// capture is one subscriber: it runs subscribe() in the background and records
// every byte the hub hands it.
type capture struct {
	mu                  sync.Mutex
	received            []byte
	gate                chan struct{} // when set, onData waits on it before recording
	entered             chan struct{} // closed the first time onData is called
	enteredOnce         sync.Once
	stopAfterFirstChunk bool
	readyCalls          atomic.Int32
	done                chan error
	cancel              chan struct{}
}

func startCapture(hub *avcHub, src avcSource) *capture {
	return newCapture().run(hub, src)
}

func startCaptureThatStopsAfterFirstChunk(hub *avcHub, src avcSource) *capture {
	c := newCapture()
	c.stopAfterFirstChunk = true
	return c.run(hub, src)
}

// startBlockedCapture starts a subscriber whose onData does not return until the
// returned release func is called.
func startBlockedCapture(hub *avcHub, src avcSource) (*capture, func()) {
	c := newCapture()
	c.gate = make(chan struct{})
	return c.run(hub, src), func() { close(c.gate) }
}

func newCapture() *capture {
	return &capture{done: make(chan error, 1), cancel: make(chan struct{}), entered: make(chan struct{})}
}

func (c *capture) run(hub *avcHub, src avcSource) *capture {
	inner := src.ready
	src.ready = func() {
		c.readyCalls.Add(1)
		if inner != nil {
			inner()
		}
	}
	go func() { c.done <- hub.subscribe(src, c.onData, c.cancel) }()
	return c
}

// wasReportedReady tells whether the hub told this capture its stream is live.
func (c *capture) wasReportedReady() bool { return c.readyCalls.Load() > 0 }

// waitUntilHandlingAChunk returns once the subscriber is inside onData, so a
// test knows that chunk left the queue before it floods the rest.
func (c *capture) waitUntilHandlingAChunk(t *testing.T) {
	t.Helper()
	select {
	case <-c.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber never started handling a chunk")
	}
}

func (c *capture) onData(chunk []byte) bool {
	c.enteredOnce.Do(func() { close(c.entered) })
	if c.gate != nil {
		<-c.gate
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.received = append(c.received, chunk...)
	return !c.stopAfterFirstChunk
}

func (c *capture) interrupt() { close(c.cancel) }

func (c *capture) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return bytes.Clone(c.received)
}

func (c *capture) waitForBytes(t *testing.T, n int) []byte {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		got := c.bytes()
		if len(got) >= n {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d bytes, got %d", n, len(got))
		}
		time.Sleep(time.Millisecond)
	}
}

func (c *capture) waitUntilFinished(t *testing.T) error {
	t.Helper()
	select {
	case err := <-c.done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the subscriber to finish")
		return nil
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func expectBytes(t *testing.T, got, want []byte, what string) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s:\n got %x\nwant %x", what, got, want)
	}
}

// --- tests ------------------------------------------------------------------

func TestFirstSubscriberReceivesEveryByteFromTheStart(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	first := startCapture(hub, source.source())
	source.waitUntilStarted(t)

	stream := concat(deltaFrameNAL(1), spsNAL(), ppsNAL(), keyFrameNAL(2), deltaFrameNAL(3))
	source.feed(stream)

	expectBytes(t, first.waitForBytes(t, len(stream)), stream, "first subscriber")
}

func TestLateJoinerStartsAtTheParameterSetsAndNextKeyFrame(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	first := startCapture(hub, source.source())
	source.waitUntilStarted(t)
	firstGOP := concat(spsNAL(), ppsNAL(), keyFrameNAL(1), deltaFrameNAL(2))
	source.feed(firstGOP)
	first.waitForBytes(t, len(firstGOP))

	late := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)

	// mid-GOP bytes must never reach the late joiner
	source.feed(deltaFrameNAL(3))
	source.feed(concat(keyFrameNAL(4), deltaFrameNAL(5)))

	wantLate := concat(spsNAL(), ppsNAL(), keyFrameNAL(4), deltaFrameNAL(5))
	expectBytes(t, late.waitForBytes(t, len(wantLate)), wantLate, "late joiner")
}

func TestBothSubscribersReceiveIdenticalBytesAfterTheJoin(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	first := startCapture(hub, source.source())
	source.waitUntilStarted(t)
	beforeJoin := concat(spsNAL(), ppsNAL(), keyFrameNAL(1), deltaFrameNAL(2))
	source.feed(beforeJoin)

	late := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)

	afterJoin := concat(keyFrameNAL(3), deltaFrameNAL(4), deltaFrameNAL(5), deltaFrameNAL(6))
	source.feed(afterJoin)

	parameterSets := concat(spsNAL(), ppsNAL())
	firstBytes := first.waitForBytes(t, len(beforeJoin)+len(afterJoin))
	lateBytes := late.waitForBytes(t, len(parameterSets)+len(afterJoin))

	expectBytes(t, firstBytes, concat(beforeJoin, afterJoin), "first subscriber")
	expectBytes(t, lateBytes[len(parameterSets):], afterJoin, "late joiner from the key frame on")
}

func TestLastSubscriberLeavingStopsTheSource(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	staying := startCapture(hub, source.source())
	source.waitUntilStarted(t)

	leaving := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)
	source.feed(concat(spsNAL(), ppsNAL(), keyFrameNAL(1), deltaFrameNAL(2)))
	leaving.waitForBytes(t, 1)

	leaving.interrupt()
	if err := leaving.waitUntilFinished(t); err != nil {
		t.Fatalf("interrupted subscriber returned %v, want nil", err)
	}
	source.expectStillRunning(t)

	staying.interrupt()
	if err := staying.waitUntilFinished(t); err != nil {
		t.Fatalf("last subscriber returned %v, want nil", err)
	}
	source.waitUntilStopped(t)
}

func TestASilentSourceStillLetsASubscriberStopPromptly(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	only := startCapture(hub, source.source())
	source.waitUntilStarted(t)

	// an idle android virtual display emits very few frames, so stopping must
	// not wait for the next one to arrive
	only.interrupt()

	if err := only.waitUntilFinished(t); err != nil {
		t.Fatalf("stopped subscriber returned %v, want nil", err)
	}
	source.waitUntilStopped(t)
}

func TestWatchStopCancelsWhenTheCallerStops(t *testing.T) {
	stop := make(chan struct{})
	cancelled, stopWatching := watchStop(stop)
	defer stopWatching()

	close(stop)
	waitForSignal(t, cancelled, "cancellation")
}

func TestWatchStopWithoutAStopChannelWaitsForASignalOnly(t *testing.T) {
	cancelled, stopWatching := watchStop(nil)
	defer stopWatching()

	select {
	case <-cancelled:
		t.Fatal("a capture with no stop channel was cancelled without a signal")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestResubscribingAfterTheSourceStoppedStartsItAgain(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	first := startCaptureThatStopsAfterFirstChunk(hub, source.source())
	source.waitUntilStarted(t)
	source.feed(deltaFrameNAL(1))
	if err := first.waitUntilFinished(t); err != nil {
		t.Fatalf("first subscriber returned %v, want nil", err)
	}
	source.waitUntilStopped(t)

	// the fresh source has no cached parameter sets, so this subscriber is a
	// first subscriber again and gets everything from byte 0
	second := startCapture(hub, source.source())
	source.waitUntilStarted(t)
	if got := source.startCount(); got != 2 {
		t.Fatalf("source started %d times, want 2", got)
	}

	source.feed(deltaFrameNAL(9))
	expectBytes(t, second.waitForBytes(t, len(deltaFrameNAL(9))), deltaFrameNAL(9), "restarted stream")
}

func TestSlowSubscriberIsDroppedWithoutStallingTheOther(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	slow, releaseSlow := startBlockedCapture(hub, source.source())
	source.waitUntilStarted(t)

	fast := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)
	source.feed(concat(spsNAL(), ppsNAL(), keyFrameNAL(1)))

	// overflow the slow subscriber's queue while it sits inside onData. feed in
	// batches the fast subscriber can drain, or on a single cpu the feeder
	// outruns it and the hub rightly drops both
	const batch = 32
	fed := len(keyFrameNAL(1))
	for i := 0; i < avcSubscriberQueue+batch; i += batch {
		frames := make([][]byte, batch)
		for j := range frames {
			frames[j] = deltaFrameNAL(byte(i + j))
			fed += len(frames[j])
		}
		source.feed(frames...)
		// the fast subscriber kept flowing the whole time
		fast.waitForBytes(t, fed)
	}

	releaseSlow()
	if err := slow.waitUntilFinished(t); !errors.Is(err, errAvcSubscriberTooSlow) {
		t.Fatalf("slow subscriber returned %v, want %v", err, errAvcSubscriberTooSlow)
	}
}

func TestStartCodeSplitAcrossChunksStillAlignsALateJoiner(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	first := startCapture(hub, source.source())
	source.waitUntilStarted(t)

	late := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)

	gop := concat(spsNAL(), ppsNAL(), keyFrameNAL(1), deltaFrameNAL(2))
	source.feed(splitIntoSingleBytes(gop)...)
	source.feed(deltaFrameNAL(3)) // completes the trailing NAL unit

	wantLate := concat(spsNAL(), ppsNAL(), keyFrameNAL(1), deltaFrameNAL(2), deltaFrameNAL(3))
	expectBytes(t, late.waitForBytes(t, len(wantLate)), wantLate, "late joiner over split chunks")
	expectBytes(t, first.waitForBytes(t, len(gop)+len(deltaFrameNAL(3))), concat(gop, deltaFrameNAL(3)), "first subscriber over split chunks")
}

// the timecode SEI that stamps a picture comes before its slice; a joiner that
// started at the slice would hand the mp4 muxer a key frame with no timestamp
func TestLateJoinerReceivesTheTimecodeThatPrecedesItsKeyFrame(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	first := startCapture(hub, source.source())
	source.waitUntilStarted(t)
	source.feed(concat(spsNAL(), ppsNAL(), timecodeNAL(1), keyFrameNAL(1), timecodeNAL(2), deltaFrameNAL(2)))
	first.waitForBytes(t, 1)

	late := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)

	// the head of the key frame arrives in its own chunk, as it does over tcp
	source.feed(concat(spsNAL(), ppsNAL(), timecodeNAL(3)))
	source.feed(concat(keyFrameNAL(3), timecodeNAL(4)))
	source.feed(deltaFrameNAL(4)) // completes the trailing NAL unit

	wantLate := concat(spsNAL(), ppsNAL(), timecodeNAL(3), keyFrameNAL(3), timecodeNAL(4), deltaFrameNAL(4))
	expectBytes(t, late.waitForBytes(t, len(wantLate)), wantLate, "late joiner starts at its key frame's head")
}

// a recording stopped and started again at once, while a live view keeps the
// source running, must begin with parameter sets and a key frame even when
// the encoder does not repeat SPS/PPS in front of it
func TestARecordingRestartedWhileTheLiveViewStaysStartsDecodable(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	liveView := startCapture(hub, source.source())
	source.waitUntilStarted(t)
	source.feed(concat(spsNAL(), ppsNAL(), timecodeNAL(1), keyFrameNAL(1), timecodeNAL(2), deltaFrameNAL(2)))
	liveView.waitForBytes(t, 1)

	recording := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)
	source.feed(concat(timecodeNAL(3), keyFrameNAL(3), timecodeNAL(4)))
	recording.waitForBytes(t, 1)
	recording.interrupt()
	if err := recording.waitUntilFinished(t); err != nil {
		t.Fatalf("stopped recording returned %v, want nil", err)
	}
	source.expectStillRunning(t)

	restarted := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)
	source.feed(deltaFrameNAL(4)) // mid-GOP, undecodable on its own
	source.feed(concat(timecodeNAL(5), keyFrameNAL(5), timecodeNAL(6)))
	source.feed(deltaFrameNAL(6)) // completes the trailing NAL unit

	want := concat(spsNAL(), ppsNAL(), timecodeNAL(5), keyFrameNAL(5), timecodeNAL(6), deltaFrameNAL(6))
	expectBytes(t, restarted.waitForBytes(t, len(want)), want, "restarted recording")
}

func TestSourceErrorEndsEverySubscriber(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	first := startCapture(hub, source.source())
	source.waitUntilStarted(t)

	late := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)

	sourceErr := errors.New("stream died")
	source.die(sourceErr)

	if err := first.waitUntilFinished(t); !errors.Is(err, sourceErr) {
		t.Fatalf("first subscriber returned %v, want %v", err, sourceErr)
	}
	if err := late.waitUntilFinished(t); !errors.Is(err, sourceErr) {
		t.Fatalf("late joiner returned %v, want %v", err, sourceErr)
	}
}

// hubWithQuickKeyFrameWait is a hub whose late joiners give up on a missing key
// frame in milliseconds instead of seconds.
func hubWithQuickKeyFrameWait() *avcHub {
	return &avcHub{keyFrameTimeout: 200 * time.Millisecond, keyFrameRetry: 10 * time.Millisecond}
}

func TestASubscriberCanGiveUpWhileAnotherSubscribersStartIsStillRunning(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()
	finishTheSlowStart := source.blockStart()

	// on a real iOS device this start sits in the broadcast picker for ~10s
	first := startCapture(hub, source.source())
	source.waitUntilStartBegan(t)

	impatient := startCapture(hub, source.source())
	impatient.interrupt()
	if err := impatient.waitUntilFinished(t); err != nil {
		t.Fatalf("subscriber cancelled during someone else's start returned %v, want nil", err)
	}

	finishTheSlowStart()
	source.waitUntilStarted(t)
	if got := source.startCount(); got != 1 {
		t.Fatalf("source started %d times, want 1", got)
	}

	source.feed(deltaFrameNAL(1))
	expectBytes(t, first.waitForBytes(t, len(deltaFrameNAL(1))), deltaFrameNAL(1), "the subscriber that waited out the start")
}

func TestALateJoinerGivesUpWhenNoKeyFrameEverArrives(t *testing.T) {
	hub, source := hubWithQuickKeyFrameWait(), newFakeAvcSource()
	source.neverProducesKeyFrames(errors.New("encoder refused"))

	staying := startCapture(hub, source.source())
	source.waitUntilStarted(t)
	source.feed(concat(spsNAL(), ppsNAL(), keyFrameNAL(1)))
	staying.waitForBytes(t, 1)

	// no further key frame is ever produced, so the joiner must fail rather than
	// block its caller forever
	late := startCapture(hub, source.source())
	if err := late.waitUntilFinished(t); !errors.Is(err, errAvcNoKeyFrame) {
		t.Fatalf("late joiner returned %v, want %v", err, errAvcNoKeyFrame)
	}
	if late.wasReportedReady() {
		t.Fatal("a late joiner that never received a key frame was reported ready")
	}
	if got := source.keyFrameRequestCount(); got < 2 {
		t.Fatalf("key frame was requested %d times, want it retried at least twice", got)
	}
	source.expectStillRunning(t)
}

func TestALateJoinerIsOnlyReportedReadyOnceItsKeyFrameArrives(t *testing.T) {
	hub, source := hubWithQuickKeyFrameWait(), newFakeAvcSource()

	startCapture(hub, source.source())
	source.waitUntilStarted(t)
	source.feed(concat(spsNAL(), ppsNAL(), keyFrameNAL(1)))

	late := startCapture(hub, source.source())
	source.waitForKeyFrameRequest(t)
	if late.wasReportedReady() {
		t.Fatal("a late joiner was reported ready before its first key frame")
	}

	source.feed(concat(keyFrameNAL(2), deltaFrameNAL(3)))
	late.waitForBytes(t, 1)
	if !late.wasReportedReady() {
		t.Fatal("a late joiner receiving bytes was never reported ready")
	}
}

func TestANewSubscriberWaitsForTheOldSourceToFinishStopping(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()
	finishTheSlowStop := source.blockStop()

	first := startCapture(hub, source.source())
	source.waitUntilStartBegan(t)
	source.waitUntilStarted(t)
	first.interrupt()
	source.waitUntilStopBegan(t)

	// a second iOS dial here would steal the broadcast extension's stream from
	// the source that is still closing its conn
	second := startCapture(hub, source.source())
	source.expectNotStartedAgain(t)

	finishTheSlowStop()
	source.waitUntilStarted(t)
	if got := source.mostSourcesAliveAtOnce(); got != 1 {
		t.Fatalf("%d sources were alive at once, want 1", got)
	}

	source.feed(deltaFrameNAL(7))
	expectBytes(t, second.waitForBytes(t, len(deltaFrameNAL(7))), deltaFrameNAL(7), "the subscriber that waited out the stop")
}

func TestADroppedSubscriberStopsWithoutDrainingItsStaleQueue(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	slow, releaseSlow := startBlockedCapture(hub, source.source())
	source.waitUntilStarted(t)

	firstChunk := concat(spsNAL(), ppsNAL(), keyFrameNAL(1))
	source.feed(firstChunk)
	slow.waitUntilHandlingAChunk(t)

	backlog := make([][]byte, 0, avcSubscriberQueue+16)
	for i := 0; i < avcSubscriberQueue+16; i++ {
		backlog = append(backlog, deltaFrameNAL(byte(i)))
	}
	source.feed(backlog...)

	releaseSlow()
	if err := slow.waitUntilFinished(t); !errors.Is(err, errAvcSubscriberTooSlow) {
		t.Fatalf("slow subscriber returned %v, want %v", err, errAvcSubscriberTooSlow)
	}
	// it left right after the chunk it was already inside, instead of writing
	// out a queue full of stale ones
	expectBytes(t, slow.bytes(), firstChunk, "dropped subscriber")
}

func TestConcurrentFirstSubscribersStartTheSourceOnce(t *testing.T) {
	hub, source := &avcHub{}, newFakeAvcSource()

	const subscribers = 5
	for i := 0; i < subscribers; i++ {
		startCapture(hub, source.source())
	}

	source.waitUntilStarted(t)
	for i := 0; i < subscribers-1; i++ {
		source.waitForKeyFrameRequest(t)
	}
	if got := source.startCount(); got != 1 {
		t.Fatalf("source started %d times, want 1", got)
	}
}
