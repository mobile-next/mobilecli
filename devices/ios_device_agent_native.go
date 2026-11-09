package devices

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/mobile-next/mobilecli/agents"
	"github.com/mobile-next/mobilecli/devices/ios/machomap"
	"github.com/mobile-next/mobilecli/utils"
)

// injectTimeout bounds the whole debugserver map. The map normally takes a few
// seconds; this is only a ceiling so a wedged device (a debugserver reply that
// never arrives) cannot hang the CLI/daemon indefinitely.
const injectTimeout = 60 * time.Second

const (
	injectAttempts   = 3
	injectRetryDelay = time.Second
)

var errAttachRefused = errors.New("only debuggable apps can be injected, so system apps, App Store apps and release builds are not supported")

// errAppBusyDuringProbe means the app's thread trapped while being probed for an
// existing agent. It was seen on about 1 in 15 first injections while a keyboard
// was opening. The cause is not known, and attaching again has worked.
var errAppBusyDuringProbe = errors.New("app trapped during the agent probe")

// injectAgentNative maps the compiled agent dylib into the foreground app over
// the developer debugserver and runs its initializer, with no code signing and
// no re-signing (the CS_DEBUGGED exemption lets the debugger execute the pages
// it wrote). It replaces the old LLDB `expr -l objc` source-injection path and
// works where dlopen does not (AWS Device Farm). Returns the device-side TCP port
// the agent's server bound.
func injectAgentNative(device goios.DeviceEntry, proxyPort, pid int) (int, error) {
	return retryWhileAppIsBusy(func() (int, error) {
		return injectAgentOnce(device, proxyPort, pid)
	})
}

// retryWhileAppIsBusy attaches again when the probe trapped.
func retryWhileAppIsBusy(inject func() (int, error)) (int, error) {
	for attempt := 1; ; attempt++ {
		port, err := inject()
		if !shouldRetryInjection(err, attempt) {
			return port, err
		}
		utils.Verbose("attempt %d: %v", attempt, err)
		time.Sleep(injectRetryDelay)
	}
}

// shouldRetryInjection also retries a refused attach, but only after a first
// attempt: debugserver can refuse a new attach just after the previous detach. A
// refused first attempt means the app is not debuggable, so it fails at once.
func shouldRetryInjection(err error, attempt int) bool {
	if attempt == injectAttempts {
		return false
	}
	return errors.Is(err, errAppBusyDuringProbe) || (attempt > 1 && errors.Is(err, errAttachRefused))
}

// probeError marks a trap in the probe as safe to retry. Only the probe is
// retried: it runs before the agent is mapped, while a failed map is half-written.
func probeError(err error) error {
	if errors.Is(err, machomap.ErrUnexpectedStop) {
		return fmt.Errorf("%w: %w", errAppBusyDuringProbe, err)
	}
	return fmt.Errorf("probe existing agent: %w", err)
}

func injectAgentOnce(device goios.DeviceEntry, proxyPort, pid int) (int, error) {
	conn, err := goios.ConnectTUNDevice(device.Address, proxyPort, device)
	if err != nil {
		return 0, fmt.Errorf("connect debug proxy: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Bound the blocking RSP exchanges: closing the connection when the deadline
	// expires unblocks any pending read in mapAgentOverConn, which then returns an
	// error instead of hanging.
	var timedOut atomic.Bool
	timer := time.AfterFunc(injectTimeout, func() { timedOut.Store(true); _ = conn.Close() })
	defer timer.Stop()

	port, err := mapAgentOverConn(conn, pid)
	if timedOut.Load() {
		return 0, fmt.Errorf("agent injection timed out after %s (device unresponsive)", injectTimeout)
	}
	return port, err
}

// attachError explains a refused attach. debugserver answers E96 when the
// process is not debuggable, which is the normal case for system and release apps.
func attachError(pid int, response string) error {
	if strings.HasPrefix(response, "E") {
		return fmt.Errorf("cannot attach to pid %d (debugserver %s): %w", pid, response, errAttachRefused)
	}
	return fmt.Errorf("vAttach pid %d: unexpected response %q", pid, response)
}

// mapAgentOverConn runs the attach + map + boot sequence on an already-opened
// debugserver stream. Split out from injectAgentNative so the logic can be tested
// over any transport (e.g. the go-ios userspace-tunnel forwarder).
func mapAgentOverConn(conn io.ReadWriter, pid int) (int, error) {
	rsp, err := machomap.NewRSP(conn)
	if err != nil {
		return 0, fmt.Errorf("debugserver handshake: %w", err)
	}
	if r, err := rsp.Request("QSetDetachOnError:1"); err != nil || r != "OK" {
		return 0, fmt.Errorf("QSetDetachOnError: resp=%q err=%v", r, err)
	}
	stop, err := rsp.Request(fmt.Sprintf("vAttach;%x", pid))
	if err != nil {
		return 0, fmt.Errorf("vAttach pid %d: %w", pid, err)
	}
	if !strings.HasPrefix(stop, "T") {
		return 0, attachError(pid, stop)
	}
	defer func() { _, _ = rsp.Request("D") }()

	resolver := machomap.NewExportTrieResolver(rsp)

	// Idempotency: never map the same image twice into one process — duplicate
	// ObjC class registration fails and leaves the image half-wired. If the agent
	// is already present (e.g. after a daemon restart while the app kept running),
	// the in-process port cache is gone and the agent's ephemeral port cannot be
	// recovered, so ask the caller to relaunch the app.
	if already, err := machomap.ClassIsRegistered(rsp, resolver, "MobileServer"); err != nil {
		return 0, probeError(err)
	} else if already {
		return 0, fmt.Errorf("agent already injected in pid %d; relaunch the app to re-inject", pid)
	}

	res, err := machomap.Map(rsp, agents.IOSAgentDevDylib, resolver, machomap.Options{RunInit: true})
	if err != nil {
		return 0, fmt.Errorf("map agent dylib: %w", err)
	}
	utils.Verbose("native map: base=%#x size=%#x rebases=%d binds=%d classes=%d categories=%d",
		res.Base, res.ImageSize, res.Rebases, res.Binds, res.Classes, res.Categories)

	// Read the server's bound port from the agent's exported getter.
	m, err := machomap.ParseMacho(agents.IOSAgentDevDylib)
	if err != nil {
		return 0, err
	}
	off, ok := m.Symbols["_mobilecli_get_port"]
	if !ok {
		return 0, fmt.Errorf("agent dylib missing mobilecli_get_port")
	}
	port, err := machomap.CallFunction(rsp, res.Base+off)
	if err != nil {
		return 0, fmt.Errorf("read agent port: %w", err)
	}
	p := int(uint32(port)) // #nosec G115 -- the function returns a C int in a 64-bit register
	if p == 0 {
		return 0, fmt.Errorf("agent initializer ran but server did not bind")
	}
	utils.Verbose("native map: agent server bound device port %d", p)
	return p, nil
}
