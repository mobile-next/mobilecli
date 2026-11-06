package devices

import (
	"reflect"
	"testing"
)

func TestParseSimulatorListeningPorts(t *testing.T) {
	output := "COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME\n" +
		"playgroun 92664 gilm 11u IPv4 0x1 0t0 TCP 192.168.0.101:55975->13.225.245.14:443 (ESTABLISHED)\n" +
		"playgroun 92664 gilm 13u IPv4 0x2 0t0 TCP 127.0.0.1:56022 (LISTEN)\n" +
		"playgroun 92664 gilm 14u IPv4 0x3 0t0 TCP 0.0.0.0:3000 (LISTEN)\n"
	if got := parseSimulatorListeningPorts(output); !reflect.DeepEqual(got, []int{56022}) {
		t.Fatalf("ports = %v, want [56022]", got)
	}
}
