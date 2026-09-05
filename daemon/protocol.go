package daemon

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// Wire format: newline-delimited JSON-RPC 2.0, one request per connection.
// Notifications (no id) may precede the final response on streaming methods.

const jsonrpcVersion = "2.0"

// StreamMethod is the notification method used for stream payloads.
const StreamMethod = "stream.data"

// CancelMethod is the notification a client sends mid-call to cancel the
// handler's context while keeping the connection open for the final result.
const CancelMethod = "cancel"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"` // set on notifications only
	Params  json.RawMessage `json:"params,omitempty"` // notification payload
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC error object returned by the daemon.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error prefers the detailed data string when present, matching what the CLI
// printed before the daemon existed.
func (e *RPCError) Error() string {
	if s, ok := e.Data.(string); ok && s != "" {
		return s
	}
	return e.Message
}

func writeFrame(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// readFrame reads one line. bufio.Scanner is deliberately avoided: its default
// token limit is 64 KB and screenshots are larger than that.
func readFrame(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		if err == io.EOF && len(line) > 0 {
			return line, nil
		}
		return nil, fmt.Errorf("read frame: %w", err)
	}
	return line, nil
}

// StreamChunk is one decoded stream.data payload: exactly one of Bytes
// (binary data), Progress (human-readable progress text) or Line (any other
// JSON object, e.g. a log entry) is set.
type StreamChunk struct {
	Bytes    []byte
	Progress string
	Line     json.RawMessage
}

// StreamData is the wire shape for binary stream payloads.
type StreamData struct {
	Data string `json:"data"` // base64
}

// StreamProgress is the wire shape for progress text.
type StreamProgress struct {
	Progress string `json:"progress"`
}

// DecodeStreamChunk classifies a stream.data payload for consumers.
func DecodeStreamChunk(payload json.RawMessage) (StreamChunk, error) {
	var wire struct {
		Data     *string `json:"data"`
		Progress *string `json:"progress"`
	}
	if err := json.Unmarshal(payload, &wire); err != nil {
		return StreamChunk{Line: payload}, nil
	}
	switch {
	case wire.Data != nil:
		b, err := base64.StdEncoding.DecodeString(*wire.Data)
		if err != nil {
			return StreamChunk{}, fmt.Errorf("invalid stream chunk: %w", err)
		}
		return StreamChunk{Bytes: b}, nil
	case wire.Progress != nil:
		return StreamChunk{Progress: *wire.Progress}, nil
	default:
		return StreamChunk{Line: payload}, nil
	}
}
