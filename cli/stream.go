package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mobile-next/mobilecli/daemon"
)

// streamViaDaemon runs a streaming CLI method: binary chunks go to stdout,
// progress text to stderr, anything else is printed as one NDJSON line. The
// final envelope is printed only when it reports an error, matching the
// pre-daemon behavior of logs/screencapture/screenrecord.
func streamViaDaemon(method string, params any) error {
	if err := ensureDaemon(); err != nil {
		printJson(errorEnvelope(err))
		return err
	}

	raw, err := daemon.CallStream(cmdContext(), method, params, func(payload json.RawMessage) error {
		return writeStreamPayload(payload, os.Stdout, os.Stderr)
	})
	if err != nil {
		if isUserCancellation(err) {
			return nil
		}
		printJson(errorEnvelope(err))
		return err
	}
	if err := envelopeError(raw); err != nil {
		printJson(errorEnvelope(err))
		return err
	}
	return nil
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
