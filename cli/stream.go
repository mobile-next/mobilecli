package cli

import (
	"encoding/base64"
	"encoding/json"
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
	return err != nil && (err.Error() == "context canceled" || err.Error() == "context deadline exceeded")
}

func writeStreamPayload(payload json.RawMessage, stdout, stderr io.Writer) error {
	var chunk struct {
		Data     *string `json:"data"`
		Progress *string `json:"progress"`
	}
	if err := json.Unmarshal(payload, &chunk); err == nil {
		if chunk.Data != nil {
			bytes, err := base64.StdEncoding.DecodeString(*chunk.Data)
			if err != nil {
				return fmt.Errorf("invalid stream chunk: %w", err)
			}
			_, err = stdout.Write(bytes)
			return err
		}
		if chunk.Progress != nil {
			_, err := io.WriteString(stderr, *chunk.Progress)
			return err
		}
	}
	_, err := fmt.Fprintf(stdout, "%s\n", payload)
	return err
}
