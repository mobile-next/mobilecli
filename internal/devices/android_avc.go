package devices

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mobile-next/mobilecli/internal/agentassets"
	"github.com/mobile-next/mobilecli/internal/avc2mp4"
	"github.com/mobile-next/mobilecli/internal/utils"
)

// avcServerClass is the agent class that streams the screen as raw H.264.
const avcServerClass = "com.mobilenext.mobilecli.AvcServer"

// avcServerStartGrace is how long a start waits for the first video before it
// reports the capture live anyway. Startup failures (codec refused, control
// socket taken) show within it; a dozing or static screen may draw nothing for
// longer, and that is not a failure: the stream stays open until video comes.
const avcServerStartGrace = 5 * time.Second

// avcServerCleanupTimeout bounds each adb command that kills an AvcServer on
// the device, so an unresponsive adb cannot hold up a stop.
const avcServerCleanupTimeout = 2 * time.Second

// avcServerReadSize is how much of the stream is read, and published, at once.
const avcServerReadSize = 64 << 10

// avcServerPreambleLimit is how much text may come before the video. adb
// exec-out carries the server's stderr on the same stream, and some devices
// print linker or app_process warnings first.
const avcServerPreambleLimit = 4 << 10

// avcControlSocketTaken is the end of the error AvcServer prints when another
// server holds its control socket; must match AvcServer.stream().
const avcControlSocketTaken = avcControlSocket + " is taken"

var (
	errAvcServerExited  = errors.New("the screen capture agent stopped")
	errAvcServerNoVideo = errors.New("the screen capture agent printed no video")
)

// avcServerProcess is one running AvcServer, as the host sees it.
type avcServerProcess struct {
	stdout  io.ReadCloser   // the raw H.264 stream
	exited  <-chan struct{} // closed once the host-side process is gone
	exitErr func() error    // why it exited; read only after exited is closed
	kill    func()          // ends it on the host and on the device, returns once both are gone
}

// startSharedAvcCapture subscribes config's capture to the device's one shared
// AvcServer, starting the server when nobody else is capturing.
//
// Like real iOS devices, all concurrent avc captures of a device (a live view,
// a recording, ...) share one encoder: AvcServer's control socket has a fixed
// name, so a second server would take the first one's keyframe and bitrate
// requests. The first capture's scale, fps and bitrate start the server; later
// captures join it as it runs and their settings are ignored. A bitrate change
// made through SetAvcBitrate applies to every capture of the device.
func (d *AndroidDevice) startSharedAvcCapture(config ScreenCaptureConfig) error {
	cancel, stopWatching := watchStop(config.StopChan)
	defer stopWatching()

	err := d.avcStream.subscribe(avcSource{
		start: func(emit func([]byte), ended func(error)) (func(), error) {
			launch := func() (*avcServerProcess, error) { return d.launchAvcServer(config) }
			return runAvcServerRetrying(launch, d.killLeftoverAvcServers, avcServerStartGrace, emit, ended)
		},
		ready:           config.OnReady,
		requestKeyFrame: func() error { return RequestAvcKeyFrame(d) },
	}, config.OnData, cancel)

	utils.Verbose("avc stream ended")
	return err
}

// runAvcServerRetrying is runAvcServer, retried once after killing leftover
// servers when the control socket was taken: the kill before the launch can
// race a leftover that is still shutting down.
func runAvcServerRetrying(launch func() (*avcServerProcess, error), killLeftovers func(), grace time.Duration, emit func([]byte), ended func(error)) (func(), error) {
	stop, err := runAvcServer(launch, grace, emit, ended)
	if err == nil || !strings.Contains(err.Error(), avcControlSocketTaken) {
		return stop, err
	}

	utils.Info("the screen capture control socket is taken; killing leftover servers and retrying once")
	killLeftovers()
	return runAvcServer(launch, grace, emit, ended)
}

// runAvcServer launches an AvcServer and pumps its stdout into emit until it
// exits, then calls ended. It returns once video starts, or after grace while
// the server stays silent; a server that exits or prints only text before then
// fails the start with its own message. The returned stop kills the server; it
// does not wait for the reading goroutine, so it may be called from inside emit.
func runAvcServer(launch func() (*avcServerProcess, error), grace time.Duration, emit func([]byte), ended func(error)) (func(), error) {
	proc, err := launch()
	if err != nil {
		return nil, err
	}
	stop := sync.OnceFunc(proc.kill)

	// both buffered and each sent at most once, so neither side blocks on the other
	outcome := make(chan error, 1)
	decision := make(chan bool, 1)
	go pumpAvcServer(proc, stop, outcome, decision, emit, ended)

	timer := time.NewTimer(grace)
	defer timer.Stop()

	select {
	case err := <-outcome:
		if err != nil {
			decision <- false
			stop()
			return nil, err
		}
	case <-timer.C:
		utils.Verbose("screen capture agent has no video yet after %s, keeping the stream open", grace)
	}

	decision <- true
	return stop, nil
}

// pumpAvcServer reads the server's stdout. Whether video started is reported on
// outcome; runAvcServer's decision says whether the start still stands, and
// nothing is emitted before it does.
func pumpAvcServer(proc *avcServerProcess, stop func(), outcome chan<- error, decision <-chan bool, emit func([]byte), ended func(error)) {
	video, text, err := readUntilVideo(proc.stdout, avcServerPreambleLimit)
	if video == nil {
		failure := avcServerStartupError(proc, text, err)
		outcome <- failure
		// true: the start already returned after its grace, so this is the end
		if <-decision {
			stop()
			ended(failure)
		}
		return
	}
	if len(text) > 0 {
		utils.Verbose("screen capture agent printed before its video: %q", text)
	}

	outcome <- nil
	if !<-decision {
		return
	}

	emit(video)
	buffer := make([]byte, avcServerReadSize)
	for {
		n, err := proc.stdout.Read(buffer)
		if n > 0 {
			emit(buffer[:n])
		}
		if err != nil {
			stop()
			ended(avcServerEndError(proc, err))
			return
		}
	}
}

// readUntilVideo reads r until an H.264 start code appears, and returns the
// bytes from it on, together with any text printed before it. With no start
// code in the first limit bytes, or when r ends first, video is nil and err
// says why.
func readUntilVideo(r io.Reader, limit int) (video []byte, text []byte, err error) {
	buffer := make([]byte, avcServerReadSize)
	var window []byte // the last bytes read, which a split start code may span
	for {
		n, readErr := r.Read(buffer)
		window = append(window, buffer[:n]...)
		if at := avc2mp4.FindStartCode(window, 0); at >= 0 {
			text = append(text, window[:at]...)
			return bytes.Clone(window[at:]), text, nil
		}

		// keep what could be the start of a start code for the next read
		keep := min(len(window), 3)
		text = append(text, window[:len(window)-keep]...)
		window = bytes.Clone(window[len(window)-keep:])
		if len(text)+len(window) > limit {
			return nil, text[:min(len(text), limit)], errAvcServerNoVideo
		}
		if readErr != nil {
			return nil, append(text, window...), readErr
		}
	}
}

// avcServerEndError says why the server's stream ended. The server never ends
// on its own while it is wanted, so even a clean exit is an error.
func avcServerEndError(proc *avcServerProcess, readErr error) error {
	<-proc.exited
	if exitErr := proc.exitErr(); exitErr != nil {
		return fmt.Errorf("%w: %w", errAvcServerExited, exitErr)
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, os.ErrClosed) {
		return fmt.Errorf("%w: %w", errAvcServerExited, readErr)
	}
	return errAvcServerExited
}

// avcServerStartupError is why a server ended, or was given up on, before any
// video: the text it printed when there is some, or how it exited.
func avcServerStartupError(proc *avcServerProcess, text []byte, readErr error) error {
	message := strings.TrimSpace(string(text))
	if message == "" {
		if errors.Is(readErr, errAvcServerNoVideo) {
			return readErr
		}
		return avcServerEndError(proc, readErr)
	}
	return fmt.Errorf("failed to start the screen capture agent: %s", message)
}

// launchAvcServer pushes the agent and starts `adb exec-out ... AvcServer` with
// config's settings. Any AvcServer already on the device is killed first: this
// device's hub is not running one, so it belongs to a capture that is gone (or
// to another program), and it would hold the control socket.
func (d *AndroidDevice) launchAvcServer(config ScreenCaptureConfig) (*avcServerProcess, error) {
	if err := d.pushTempFile(agentassets.AndroidMobilecliDEX, androidDexPath); err != nil {
		return nil, fmt.Errorf("push .dex: %w", err)
	}
	if config.OnProgress != nil {
		config.OnProgress("Starting Agent")
	}

	d.killLeftoverAvcServers()

	cmdArgs := append([]string{"-s", d.getAdbIdentifier(), "exec-out"}, avcServerArgs(config)...)
	utils.Verbose("Running command: %s %s", getAdbPath(), strings.Join(cmdArgs, " "))

	// a plain pipe rather than StdoutPipe: Wait then only waits for adb itself,
	// so the process can be reaped while the stream is still being read
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd := exec.CommandContext(context.Background(), getAdbPath(), cmdArgs...)
	cmd.Stdout = stdoutWriter
	if err := cmd.Start(); err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return nil, fmt.Errorf("failed to start %s: %w", avcServerClass, err)
	}
	_ = stdoutWriter.Close()

	exited := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(exited)
	}()

	return &avcServerProcess{
		stdout:  stdoutReader,
		exited:  exited,
		exitErr: func() error { return waitErr },
		kill: func() {
			// killing adb alone leaves the server running on the device until its
			// next write fails, which a static screen may never make
			stopAvcServer(avcServerStop{
				askToQuit:  d.askAvcServerToQuit,
				killByName: func() { d.killRemoteAvcServers(avcServerAnyPattern()) },
				killHost:   func() { _ = cmd.Process.Kill() },
				exited:     exited,
				askTimeout: avcServerAskTimeout,
				quitWait:   avcServerQuitWait,
			})
			_ = stdoutReader.Close()
		},
	}, nil
}

// avcServerArgs is the device command line that starts AvcServer with
// config's settings.
func avcServerArgs(config ScreenCaptureConfig) []string {
	args := []string{
		"CLASSPATH=" + androidDexPath, "app_process", "/", avcServerClass,
		"--quality", fmt.Sprintf("%d", config.Quality),
		"--scale", fmt.Sprintf("%.4f", config.Scale),
		"--fps", fmt.Sprintf("%d", config.FPS),
	}
	if config.Bitrate > 0 {
		args = append(args, "--bitrate", fmt.Sprintf("%d", config.Bitrate))
	}
	// One server per device: a server that cannot bind the control socket must
	// fail, so the start can clear the leftover and retry, instead of streaming
	// with no way to control or stop it.
	return append(args, "--exclusive")
}

// avcServerAnyPattern is the pgrep -f pattern for every AvcServer and the
// `sh -c` wrapper adbd runs it in. toybox pgrep matches an extended regular
// expression against /proc/<pid>/cmdline with NULs as spaces; the bracket
// keeps it from matching the shell running pgrep.
func avcServerAnyPattern() string {
	return "[" + avcServerClass[:1] + "]" + regexp.QuoteMeta(avcServerClass[1:])
}

// killLeftoverAvcServers kills every AvcServer on the device. Only called while
// this device's hub runs none, so any found was not started by this capture.
func (d *AndroidDevice) killLeftoverAvcServers() {
	if killed := d.killRemoteAvcServers(avcServerAnyPattern()); killed > 0 {
		utils.Info("killed %d screen capture server process(es) left on the device by another capture", killed)
	}
}

// killRemoteAvcServers SIGKILLs the processes matching pattern on the device
// and reports how many it signaled. Best effort: the device may be gone.
func (d *AndroidDevice) killRemoteAvcServers(pattern string) int {
	ctx, cancel := context.WithTimeout(context.Background(), avcServerCleanupTimeout)
	defer cancel()

	out, err := d.runAdbCommandContext(ctx, adbShellArgs("pgrep", "-f", pattern)...)
	pids := strings.Fields(string(out))
	if err != nil || len(pids) == 0 {
		return 0
	}

	if out, err := d.runAdbCommandContext(ctx, adbShellArgs(append([]string{"kill", "-KILL"}, pids...)...)...); err != nil {
		utils.Verbose("failed to kill the screen capture server on the device: %v\n%s", err, string(out))
		return 0
	}
	return len(pids)
}
