package devices

import (
	"strconv"
	"strings"
	"testing"
)

// AWS Device Farm runs the adb server on another host and tunnels each forwarded port by
// its number, so "tcp:0" reaches nothing there. The port has to be named.
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
