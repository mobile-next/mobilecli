package devices

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake AvcServer -----------------------------------------------------------

// fakeAvcServer stands in for one `adb exec-out ... AvcServer` process: the test
// writes what the device would print, and observes whether it was killed.
type fakeAvcServer struct {
	out      *io.PipeWriter
	stdout   *io.PipeReader
	exited   chan struct{}
	exitOnce sync.Once
	exitErr  error
	killed   atomic.Bool
}

func newFakeAvcServer() *fakeAvcServer {
	r, w := io.Pipe()
	return &fakeAvcServer{out: w, stdout: r, exited: make(chan struct{})}
}

func (f *fakeAvcServer) process() *avcServerProcess {
	return &avcServerProcess{
		stdout:  f.stdout,
		exited:  f.exited,
		exitErr: func() error { return f.exitErr },
		kill: func() {
			f.killed.Store(true)
			f.exit(nil)
		},
	}
}

// print writes bytes to the process's stdout, as the device would.
func (f *fakeAvcServer) print(b []byte) { _, _ = f.out.Write(b) }

// exit ends the process on its own, like AvcServer dying or the device going away.
func (f *fakeAvcServer) exit(err error) {
	f.exitOnce.Do(func() {
		f.exitErr = err
		_ = f.out.Close()
		close(f.exited)
	})
}

func (f *fakeAvcServer) wasKilled() bool { return f.killed.Load() }

// fakeAvcLauncher hands out one fakeAvcServer per launch and counts launches.
type fakeAvcLauncher struct {
	mu       sync.Mutex
	servers  []*fakeAvcServer
	launched chan *fakeAvcServer
}

func newFakeAvcLauncher() *fakeAvcLauncher {
	return &fakeAvcLauncher{launched: make(chan *fakeAvcServer, 16)}
}

func (l *fakeAvcLauncher) launch() (*avcServerProcess, error) {
	server := newFakeAvcServer()
	l.mu.Lock()
	l.servers = append(l.servers, server)
	l.mu.Unlock()
	l.launched <- server
	return server.process(), nil
}

func (l *fakeAvcLauncher) launchCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.servers)
}

func (l *fakeAvcLauncher) waitForLaunch(t *testing.T) *fakeAvcServer {
	t.Helper()
	select {
	case server := <-l.launched:
		return server
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an AvcServer launch")
		return nil
	}
}

// sharedAndroidSource is the avcSource an Android device hands its hub, with
// the launcher swapped for the fake. A key frame request makes the running
// server print a fresh key frame and the frame after it (the hub only knows a
// unit is complete once the next start code arrives), the way MediaCodec
// answers a sync frame request: without repeating SPS/PPS.
func sharedAndroidSource(l *fakeAvcLauncher, grace time.Duration) avcSource {
	return avcSource{
		start: func(emit func([]byte), ended func(error)) (func(), error) {
			return runAvcServer(l.launch, grace, neverCancelled, emit, ended)
		},
		requestKeyFrame: func() error {
			l.mu.Lock()
			server := l.servers[len(l.servers)-1]
			l.mu.Unlock()
			go server.print(concat(timecodeNAL(9), keyFrameNAL(9), deltaFrameNAL(10)))
			return nil
		},
	}
}

// startAvcServerInBackground runs runAvcServer on its own goroutine and hands
// back its result, so a test can play the device meanwhile.
func startAvcServerInBackground(l *fakeAvcLauncher, grace time.Duration, emit func([]byte), ended func(error)) <-chan error {
	result := make(chan error, 1)
	go func() {
		_, err := runAvcServer(l.launch, grace, neverCancelled, emit, ended)
		result <- err
	}()
	return result
}

func waitForResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the AvcServer start to finish")
		return nil
	}
}

func ignoreChunks([]byte) {}
func ignoreEnd(error)     {}

// neverCancelled is the cancel channel of a capture nobody stops.
var neverCancelled <-chan struct{}

// --- AvcServer process lifecycle --------------------------------------------------

func TestAvcServerPrintingAnErrorInsteadOfVideoFailsToStart(t *testing.T) {
	launcher := newFakeAvcLauncher()
	result := startAvcServerInBackground(launcher, time.Second, ignoreChunks, ignoreEnd)
	server := launcher.waitForLaunch(t)

	server.print([]byte("Error: Video dimensions 4000x3000 exceed codec capabilities"))
	server.exit(errors.New("exit status 1"))

	err := waitForResult(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceed codec capabilities")
	assert.True(t, server.wasKilled(), "a server that failed to start must be cleaned up")
}

func TestAvcServerExitingBeforeAnyOutputFailsToStart(t *testing.T) {
	launcher := newFakeAvcLauncher()
	result := startAvcServerInBackground(launcher, time.Second, ignoreChunks, ignoreEnd)
	server := launcher.waitForLaunch(t)

	server.exit(errors.New("exit status 1"))

	err := waitForResult(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit status 1")
}

func TestASilentAvcServerStaysOpenUntilItsVideoComes(t *testing.T) {
	// a dozing or static screen may draw nothing for a long time; that is not a failure
	launcher := newFakeAvcLauncher()
	received := make(chan []byte, 1)
	result := startAvcServerInBackground(launcher, 20*time.Millisecond, func(b []byte) { received <- bytes.Clone(b) }, ignoreEnd)
	server := launcher.waitForLaunch(t)

	require.NoError(t, waitForResult(t, result), "silence past the grace must not fail the start")
	assert.False(t, server.wasKilled())

	go server.print(spsNAL())
	select {
	case got := <-received:
		assert.Equal(t, spsNAL(), got)
	case <-time.After(2 * time.Second):
		t.Fatal("video that came after the grace never reached the stream")
	}
}

func TestAServerFailingAfterTheGraceEndsTheStreamWithItsMessage(t *testing.T) {
	launcher := newFakeAvcLauncher()
	ended := make(chan error, 1)
	result := startAvcServerInBackground(launcher, 20*time.Millisecond, ignoreChunks, func(err error) { ended <- err })
	server := launcher.waitForLaunch(t)
	require.NoError(t, waitForResult(t, result))

	server.print([]byte("Error: Failed to create virtual display"))
	server.exit(errors.New("exit status 1"))

	select {
	case err := <-ended:
		assert.Contains(t, err.Error(), "Failed to create virtual display")
	case <-time.After(2 * time.Second):
		t.Fatal("the stream never ended")
	}
	assert.True(t, server.wasKilled())
}

func TestWarningsPrintedBeforeTheVideoAreLeftOutOfTheStream(t *testing.T) {
	// adb exec-out carries stderr on the same stream as the video
	launcher := newFakeAvcLauncher()
	received := make(chan []byte, 1)
	result := startAvcServerInBackground(launcher, time.Second, func(b []byte) { received <- bytes.Clone(b) }, ignoreEnd)
	server := launcher.waitForLaunch(t)

	go server.print(concat([]byte("WARNING: linker: unsupported flags DT_FLAGS_1=0x8000001\n"), spsNAL(), ppsNAL()))

	require.NoError(t, waitForResult(t, result))
	assert.Equal(t, concat(spsNAL(), ppsNAL()), <-received)
}

func TestAStartCodeSplitAcrossReadsStillStartsTheVideo(t *testing.T) {
	stream := concat([]byte("warning\n"), spsNAL(), ppsNAL())

	reader := iotest.OneByteReader(bytes.NewReader(stream))

	video, text, err := readUntilVideo(reader, avcServerPreambleLimit)
	rest, _ := io.ReadAll(reader)

	require.NoError(t, err)
	assert.Equal(t, "warning\n", string(text))
	assert.Equal(t, concat(spsNAL(), ppsNAL()), concat(video, rest), "no video byte may be lost or added")
}

func TestMoreTextThanThePreambleLimitWithoutVideoFailsTheStart(t *testing.T) {
	text := bytes.Repeat([]byte("x"), avcServerPreambleLimit+1)

	video, _, err := readUntilVideo(bytes.NewReader(text), avcServerPreambleLimit)

	assert.Nil(t, video)
	assert.ErrorIs(t, err, errAvcServerNoVideo)
}

func TestATakenControlSocketKillsLeftoversAndRetriesOnce(t *testing.T) {
	launcher := newFakeAvcLauncher()
	var leftoverKills atomic.Int32
	result := make(chan error, 1)
	go func() {
		_, err := runAvcServerRetrying(launcher.launch, func() { leftoverKills.Add(1) }, time.Second, neverCancelled, ignoreChunks, ignoreEnd)
		result <- err
	}()

	first := launcher.waitForLaunch(t)
	first.print([]byte("Error: another screen capture is already running on this device (" + avcControlSocketTaken + ")"))
	first.exit(errors.New("exit status 1"))
	second := launcher.waitForLaunch(t)
	second.print(spsNAL())

	require.NoError(t, waitForResult(t, result))
	assert.Equal(t, int32(1), leftoverKills.Load())
	assert.Equal(t, 2, launcher.launchCount())
}

func TestAnotherStartFailureIsNotRetried(t *testing.T) {
	launcher := newFakeAvcLauncher()
	result := make(chan error, 1)
	go func() {
		_, err := runAvcServerRetrying(launcher.launch, func() {}, time.Second, neverCancelled, ignoreChunks, ignoreEnd)
		result <- err
	}()

	server := launcher.waitForLaunch(t)
	server.print([]byte("Error: No video size the encoder supports fits 0x0"))
	server.exit(errors.New("exit status 1"))

	require.Error(t, waitForResult(t, result))
	assert.Equal(t, 1, launcher.launchCount())
}

func TestAvcServerDyingMidStreamEndsTheSourceWithAnError(t *testing.T) {
	launcher := newFakeAvcLauncher()
	ended := make(chan error, 1)
	result := startAvcServerInBackground(launcher, time.Second, ignoreChunks, func(err error) { ended <- err })
	server := launcher.waitForLaunch(t)
	server.print(concat(spsNAL(), ppsNAL()))
	require.NoError(t, waitForResult(t, result))

	server.exit(errors.New("device offline"))

	select {
	case err := <-ended:
		require.Error(t, err, "a server that stopped on its own is not a clean end")
		assert.Contains(t, err.Error(), "device offline")
	case <-time.After(2 * time.Second):
		t.Fatal("source never reported the end")
	}
}

func TestStoppingTheAvcServerFromInsideItsOwnEmitDoesNotDeadlock(t *testing.T) {
	// the hub stops the source from publish() when it drops the last, too slow
	// subscriber; publish runs on the goroutine reading the server's stdout
	launcher := newFakeAvcLauncher()
	stopFunc := make(chan func(), 1)
	stopReturned := make(chan struct{})
	emit := func([]byte) {
		stop := <-stopFunc
		stop()
		close(stopReturned)
	}
	result := make(chan error, 1)
	go func() {
		stop, err := runAvcServer(launcher.launch, time.Second, neverCancelled, emit, ignoreEnd)
		stopFunc <- stop
		result <- err
	}()
	server := launcher.waitForLaunch(t)
	server.print(spsNAL())
	require.NoError(t, waitForResult(t, result))

	select {
	case <-stopReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("stop called from inside emit never returned")
	}
	assert.True(t, server.wasKilled())
}

func TestCancellingASilentAvcServerStartReturnsAtOnce(t *testing.T) {
	// a dozing screen draws nothing, so the start waits out its grace; Ctrl+C
	// must not have to wait with it
	launcher := newFakeAvcLauncher()
	cancel := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := runAvcServer(launcher.launch, time.Minute, cancel, ignoreChunks, ignoreEnd)
		result <- err
	}()
	server := launcher.waitForLaunch(t)

	close(cancel)

	assert.ErrorIs(t, waitForResult(t, result), errAvcStartCancelled)
	// the stop can win before the start has taken the launched server, which
	// is then killed in the background
	assert.Eventually(t, server.wasKilled, 2*time.Second, 5*time.Millisecond)
}

// launchSteps records what a guarded launch did, in order.
type launchSteps struct {
	mu    sync.Mutex
	steps []string
}

func (l *launchSteps) did(step string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.steps = append(l.steps, step)
}

func (l *launchSteps) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.steps...)
}

func TestAStoppedLaunchTouchesNothingOnTheDevice(t *testing.T) {
	var mu sync.Mutex
	var did launchSteps
	cancel := make(chan struct{})
	close(cancel)

	_, err := launchUnlessStopped(&mu, cancel,
		func() error { did.did("push"); return nil },
		func() { did.did("kill leftovers") },
		func() (*avcServerProcess, error) { did.did("start"); return newFakeAvcServer().process(), nil })

	assert.ErrorIs(t, err, errAvcStartCancelled)
	assert.Empty(t, did.list())
}

func TestALaunchStoppedDuringItsPushDoesNotKillTheNextCapturesServer(t *testing.T) {
	// the push runs adb and can take long; by the time it returns, the capture
	// may be gone and another one may own the device's server
	var mu sync.Mutex
	var did launchSteps
	cancel := make(chan struct{})

	_, err := launchUnlessStopped(&mu, cancel,
		func() error { did.did("push"); close(cancel); return nil },
		func() { did.did("kill leftovers") },
		func() (*avcServerProcess, error) { did.did("start"); return newFakeAvcServer().process(), nil })

	assert.ErrorIs(t, err, errAvcStartCancelled)
	assert.Equal(t, []string{"push"}, did.list())
}

func TestALaunchStoppedWhileItsServerStartsKillsThatServerBeforeLettingGo(t *testing.T) {
	var mu sync.Mutex
	cancel := make(chan struct{})
	server := newFakeAvcServer()

	_, err := launchUnlessStopped(&mu, cancel,
		func() error { return nil },
		func() {},
		func() (*avcServerProcess, error) { close(cancel); return server.process(), nil })

	assert.ErrorIs(t, err, errAvcStartCancelled)
	assert.True(t, server.wasKilled(), "killed before the next launch may begin")
}

func TestLaunchesOfOneDeviceDoNotInterleave(t *testing.T) {
	var mu sync.Mutex
	var did launchSteps
	firstPushing := make(chan struct{})
	letFirstGo := make(chan struct{})
	done := make(chan struct{}, 2)
	launch := func(name string, push func() error) {
		_, _ = launchUnlessStopped(&mu, neverCancelled, push,
			func() { did.did(name + " kill leftovers") },
			func() (*avcServerProcess, error) { did.did(name + " start"); return newFakeAvcServer().process(), nil })
		done <- struct{}{}
	}

	go launch("first", func() error { close(firstPushing); <-letFirstGo; return nil })
	<-firstPushing
	go launch("second", func() error { return nil })
	close(letFirstGo)
	<-done
	<-done

	assert.Equal(t, []string{"first kill leftovers", "first start", "second kill leftovers", "second start"}, did.list())
}

func TestCancellingAHungAvcServerLaunchReturnsAtOnceAndKillsItWhenItLands(t *testing.T) {
	// the launch runs adb, which can hang; the capture must not hang with it
	release := make(chan struct{})
	server := newFakeAvcServer()
	launch := func() (*avcServerProcess, error) {
		<-release
		return server.process(), nil
	}
	cancel := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, err := runAvcServer(launch, time.Minute, cancel, ignoreChunks, ignoreEnd)
		result <- err
	}()

	close(cancel)
	assert.ErrorIs(t, waitForResult(t, result), errAvcStartCancelled)
	assert.False(t, server.wasKilled(), "nothing to kill before the launch lands")

	close(release)
	assert.Eventually(t, server.wasKilled, 2*time.Second, 5*time.Millisecond)
}

// --- one AvcServer per device, shared through the hub ----------------------------

func TestLiveStreamAndRecordingShareOneAvcServer(t *testing.T) {
	launcher := newFakeAvcLauncher()
	src := sharedAndroidSource(launcher, time.Second)
	hub := &avcHub{}

	live := startCapture(hub, src)
	server := launcher.waitForLaunch(t)
	firstGOP := concat(spsNAL(), ppsNAL(), timecodeNAL(1), keyFrameNAL(1), timecodeNAL(2), deltaFrameNAL(2))
	server.print(firstGOP)
	waitForExactBytes(t, live, firstGOP)

	recording := startCapture(hub, src)
	// the server answers the key frame request without SPS/PPS; the hub must
	// put the cached ones in front so the recording starts decodable
	waitForExactBytes(t, recording, concat(spsNAL(), ppsNAL(), timecodeNAL(9), keyFrameNAL(9), deltaFrameNAL(10)))

	assert.Equal(t, 1, launcher.launchCount(), "a second capture must not start a second AvcServer")

	live.interrupt()
	require.NoError(t, live.waitUntilFinished(t))
	assert.False(t, server.wasKilled(), "the recording still uses the server")
	recording.interrupt()
	require.NoError(t, recording.waitUntilFinished(t))
	waitUntil(t, server.wasKilled, "the last capture leaving to kill the AvcServer")
}

func TestAFreshCaptureAfterTheLastOneLeftStartsANewAvcServer(t *testing.T) {
	launcher := newFakeAvcLauncher()
	src := sharedAndroidSource(launcher, time.Second)
	hub := &avcHub{}

	first := startCapture(hub, src)
	firstServer := launcher.waitForLaunch(t)
	firstServer.print(spsNAL())
	waitForExactBytes(t, first, spsNAL())
	first.interrupt()
	require.NoError(t, first.waitUntilFinished(t))
	waitUntil(t, firstServer.wasKilled, "the first AvcServer to be killed")

	second := startCapture(hub, src)
	secondServer := launcher.waitForLaunch(t)
	secondServer.print(spsNAL())
	waitForExactBytes(t, second, spsNAL())
	second.interrupt()
	require.NoError(t, second.waitUntilFinished(t))

	assert.Equal(t, 2, launcher.launchCount())
}

func TestAvcServerArgsUseTheFirstCapturesSettings(t *testing.T) {
	args := avcServerArgs(ScreenCaptureConfig{Quality: 80, Scale: 0.5, FPS: 24, Bitrate: 2_000_000})
	joined := strings.Join(args, " ")

	assert.Contains(t, joined, "com.mobilenext.mobilecli.AvcServer")
	assert.Contains(t, joined, "--scale 0.5000")
	assert.Contains(t, joined, "--fps 24")
	assert.Contains(t, joined, "--bitrate 2000000")
}

func TestAvcServerArgsLeaveTheBitrateToTheServerWhenNoneIsAsked(t *testing.T) {
	args := avcServerArgs(ScreenCaptureConfig{Quality: 80, Scale: 1, FPS: 30})

	assert.NotContains(t, strings.Join(args, " "), "--bitrate")
}

func TestAvcServerArgsAskForTheOnlyServerOnTheDevice(t *testing.T) {
	args := avcServerArgs(ScreenCaptureConfig{Quality: 80, Scale: 1, FPS: 30})

	assert.Contains(t, args, "--exclusive")
}

// command lines as toybox pgrep -f sees them (/proc/<pid>/cmdline, NULs as
// spaces), read from a Pixel 5 on Android 14 while a capture ran
const (
	serverCommandLine  = "app_process / com.mobilenext.mobilecli.AvcServer --quality 80 --scale 1.0000 --fps 30"
	wrapperCommandLine = "/system/bin/sh -c CLASSPATH=/data/local/tmp/mobilecli.dex 'app_process' '/' 'com.mobilenext.mobilecli.AvcServer' '--quality' '80' '--scale' '1.0000' '--fps' '30'"
)

func TestTheServerPatternMatchesTheServerAndItsShell(t *testing.T) {
	assert.True(t, remotePatternMatches(avcServerAnyPattern(), serverCommandLine))
	assert.True(t, remotePatternMatches(avcServerAnyPattern(), wrapperCommandLine))
}

func TestTheServerPatternDoesNotMatchThePgrepShellItself(t *testing.T) {
	assert.False(t, remotePatternMatches(avcServerAnyPattern(), "sh -c pgrep -f '"+avcServerAnyPattern()+"'"))
}

func TestTheServerPatternDoesNotMatchOtherAgentServers(t *testing.T) {
	assert.False(t, remotePatternMatches(avcServerAnyPattern(), "app_process / com.mobilenext.mobilecli.MjpegServer --fps 30"))
}

// --- helpers ------------------------------------------------------------------

func waitForExactBytes(t *testing.T, c *capture, want []byte) {
	t.Helper()
	waitUntil(t, func() bool { return bytes.Equal(c.bytes(), want) }, "the capture to receive the expected bytes")
}

func remotePatternMatches(pattern, commandLine string) bool {
	return regexp.MustCompile(pattern).MatchString(commandLine)
}

func waitUntil(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
