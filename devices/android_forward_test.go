package devices

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// With a remote adb server, forwarded ports are tunnelled to this host by their number, so
// "tcp:0" reaches nothing there. The port has to be named.
func TestForwardNamesItsLocalPort(t *testing.T) {
	args, port, err := forwardArgs("localabstract:mobilecli-server")

	if err != nil {
		t.Fatalf("forwardArgs: %v", err)
	}
	if port <= 0 {
		t.Fatalf("no local port chosen: %d", port)
	}
	if got := strings.Join(args, " "); got != "forward tcp:"+strconv.Itoa(port)+" localabstract:mobilecli-server" {
		t.Fatalf("unexpected adb arguments: %q", got)
	}
}

// The port is free where we probed it, but adb binds it a moment later and, with a remote
// adb server, on another host. A port that turns out to be taken must not fail the forward.
func TestForwardRetriesOnAnotherPortWhenAdbCannotBindTheFirst(t *testing.T) {
	var triedPorts []string
	adbWithFirstPortTaken := func(args ...string) ([]byte, error) {
		triedPorts = append(triedPorts, args[1])
		if len(triedPorts) == 1 {
			return []byte("adb: error: cannot bind listener: Address already in use"), errors.New("exit status 1")
		}
		return nil, nil
	}

	port, err := forwardOnFreePort(adbWithFirstPortTaken, "localabstract:mobilecli-server")

	if err != nil {
		t.Fatalf("forward failed although the second port was free: %v", err)
	}
	if len(triedPorts) != 2 || triedPorts[1] != "tcp:"+strconv.Itoa(port) {
		t.Fatalf("expected a second attempt on the returned port %d, adb saw %v", port, triedPorts)
	}
}

func TestForwardDoesNotRetryOtherAdbErrors(t *testing.T) {
	attempts := 0
	adbWithoutDevice := func(args ...string) ([]byte, error) {
		attempts++
		return []byte("adb: device offline"), errors.New("exit status 1")
	}

	_, err := forwardOnFreePort(adbWithoutDevice, "localabstract:mobilecli-server")

	if err == nil || attempts != 1 {
		t.Fatalf("expected one attempt and an error, got %d attempts, err=%v", attempts, err)
	}
}
