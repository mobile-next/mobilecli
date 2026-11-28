package devices

import (
	"time"

	"github.com/mobile-next/mobilecli/internal/utils"
)

// avcServerAskTimeout bounds the screencapture.stop request, including the adb
// forward it may need.
const avcServerAskTimeout = 1 * time.Second

// avcServerQuitWait is how long a server that accepted screencapture.stop gets
// to exit before it is killed.
const avcServerQuitWait = 1 * time.Second

// avcServerStop is how one running AvcServer is ended.
type avcServerStop struct {
	askToQuit  func() error    // screencapture.stop over the control socket
	killByName func()          // kill every AvcServer on the device
	killHost   func()          // kill the host's adb process
	exited     <-chan struct{} // closed once the host's adb process is gone
	askTimeout time.Duration
	quitWait   time.Duration
}

// stopAvcServer asks the server to quit and waits for it, and kills it (on the
// device by name, then the host's adb) when the request fails, times out (an
// older agent, a hung server, a gone socket) or the server does not exit. It
// returns once the host's adb process is gone, and never waits for the
// goroutine reading the stream. A process that is already gone is left alone.
func stopAvcServer(s avcServerStop) {
	select {
	case <-s.exited:
		return
	default:
	}

	if err := askWithin(s.askToQuit, s.askTimeout); err != nil {
		utils.Verbose("screen capture server did not take the stop request: %v", err)
	} else if waitClosed(s.exited, s.quitWait) {
		return
	}

	utils.Verbose("killing the screen capture server")
	s.killByName()
	s.killHost()
	<-s.exited
}

// askWithin runs ask, giving up on it after timeout. The result channel is
// buffered so an ask that returns late does not block.
func askWithin(ask func() error, timeout time.Duration) error {
	result := make(chan error, 1)
	go func() { result <- ask() }()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-result:
		return err
	case <-timer.C:
		return errAgentTimedOut
	}
}

// waitClosed reports whether ch closed within timeout.
func waitClosed(ch <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ch:
		return true
	case <-timer.C:
		return false
	}
}

// askAvcServerToQuit sends screencapture.stop to the device's AvcServer.
func (d *AndroidDevice) askAvcServerToQuit() error {
	port, err := d.ensureControlForward()
	if err != nil {
		return err
	}
	_, err = agentRequestWithTimeout(port, "screencapture.stop", nil, avcServerAskTimeout)
	return err
}
