package devices

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// agentOutput is what the on-device agent wrote, delivered by r.
type agentOutput struct {
	r io.Reader
}

func agentWrote(output string) agentOutput {
	return agentOutput{r: strings.NewReader(output)}
}

// agentWroteOneByteAtATime delivers the output in 1-byte reads, the worst case
// for spotting a prefix that spans several reads.
func agentWroteOneByteAtATime(output string) agentOutput {
	return agentOutput{r: iotest.OneByteReader(strings.NewReader(output))}
}

// forwardEverything forwards the agent's output and returns what reached the
// caller, and the error.
func forwardEverything(output agentOutput) (string, error) {
	var received bytes.Buffer
	err := forwardAgentStream(output.r, func(chunk []byte) bool {
		received.Write(chunk)
		return true
	})
	return received.String(), err
}

const avcStreamStart = "\x00\x00\x00\x01\x67\x42\xc0\x29"

func TestForwardAgentStream_PassesTheStreamThrough(t *testing.T) {
	stream := avcStreamStart + strings.Repeat("\x01\x02\x03", 50000)

	for name, output := range map[string]agentOutput{
		"one read":         agentWrote(stream),
		"one byte at time": agentWroteOneByteAtATime(stream),
	} {
		t.Run(name, func(t *testing.T) {
			received, err := forwardEverything(output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if received != stream {
				t.Errorf("received %d bytes, want the %d bytes of the stream", len(received), len(stream))
			}
		})
	}
}

func TestForwardAgentStream_ReturnsTheAgentErrorInsteadOfForwardingIt(t *testing.T) {
	agentError := "Error: Invalid dimensions: 0x0\n"

	for name, output := range map[string]agentOutput{
		"one read":         agentWrote(agentError),
		"one byte at time": agentWroteOneByteAtATime(agentError),
	} {
		t.Run(name, func(t *testing.T) {
			received, err := forwardEverything(output)
			if err == nil {
				t.Fatal("expected the agent's error, got none")
			}
			if err.Error() != "Invalid dimensions: 0x0" {
				t.Errorf("error = %q, want %q", err.Error(), "Invalid dimensions: 0x0")
			}
			if received != "" {
				t.Errorf("forwarded %q as stream data", received)
			}
		})
	}
}

func TestForwardAgentStream_ForwardsAnOutputShorterThanTheErrorPrefix(t *testing.T) {
	received, err := forwardEverything(agentWrote("\x00\x00"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if received != "\x00\x00" {
		t.Errorf("received %q, want %q", received, "\x00\x00")
	}
}

func TestForwardAgentStream_StopsWhenTheCallerStops(t *testing.T) {
	chunks := 0
	err := forwardAgentStream(agentWroteOneByteAtATime(avcStreamStart+"more stream").r, func([]byte) bool {
		chunks++
		return chunks < 2
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunks != 2 {
		t.Errorf("onData called %d times after asking to stop at 2", chunks)
	}
}
