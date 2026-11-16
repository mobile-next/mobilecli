package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mobile-next/mobilecli/daemon"
)

// streamViaDaemon runs a streaming CLI method: binary chunks go to stdout,
// progress text to stderr, anything else is printed as one NDJSON line. The
// final envelope is printed only when it reports an error, matching the
// pre-daemon behavior of logs/screencapture.
func streamViaDaemon(method string, params any) error {
	_, err := streamResultViaDaemon(method, params)
	return err
}

// streamResultViaDaemon is streamViaDaemon for callers that print the final
// envelope themselves: it returns the envelope when it reports success. The
// envelope is nil when the user cancelled and the daemon sent no result.
func streamResultViaDaemon(method string, params any) (json.RawMessage, error) {
	if err := ensureDaemon(); err != nil {
		printJson(errorEnvelope(err))
		return nil, err
	}

	stderr := progressWriter(os.Stderr)
	raw, err := daemon.CallStream(cmdContext(), method, params, func(payload json.RawMessage) error {
		return writeStreamPayload(payload, os.Stdout, stderr)
	})
	if err != nil {
		if isUserCancellation(err) {
			return nil, nil
		}
		printJson(errorEnvelope(err))
		return nil, err
	}
	if err := envelopeError(raw); err != nil {
		printJson(errorEnvelope(err))
		return nil, err
	}
	return raw, nil
}

// progressWriter is where progress text goes. A terminal gets it as sent, so
// "\r" redraws one line in place; anything else (a pipe, a file) gets one
// line per update.
func progressWriter(file *os.File) io.Writer {
	if isTerminal(file) {
		return file
	}
	return progressLines{out: file}
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// progressLines writes each progress update on a line of its own. The daemon
// sends whole updates, separated by "\r" or "\n", so a write never ends in
// the middle of one.
type progressLines struct {
	out io.Writer
}

func (w progressLines) Write(p []byte) (int, error) {
	updates := strings.FieldsFunc(string(p), func(r rune) bool { return r == '\r' || r == '\n' })
	for _, update := range updates {
		if _, err := io.WriteString(w.out, update+"\n"); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func isUserCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func writeStreamPayload(payload json.RawMessage, stdout, stderr io.Writer) error {
	chunk, err := daemon.DecodeStreamChunk(payload)
	if err != nil {
		return err
	}
	switch {
	case chunk.Bytes != nil:
		_, err = stdout.Write(chunk.Bytes)
	case chunk.Progress != "":
		_, err = io.WriteString(stderr, chunk.Progress)
	default:
		_, err = fmt.Fprintf(stdout, "%s\n", chunk.Line)
	}
	return err
}
