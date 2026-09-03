package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/mobile-next/mobilecli/daemon"
)

var errMethodNotFound = errors.New("method not found")

// rpcTimeout bounds a forwarded call, reusing the HTTP write-deadline table
// for methods known to be slow.
func rpcTimeout(method string) time.Duration {
	if d, ok := extendedWriteDeadline(method); ok {
		return d
	}
	return daemon.DefaultCallTimeout
}

// dispatchRPC runs a JSON-RPC method: HTTP-only methods (session tickets,
// server lifecycle) run here; everything else is forwarded to the daemon,
// which owns the devices. Unknown methods are rejected before forwarding so
// the caller gets a proper -32601.
func dispatchRPC(method string, params json.RawMessage) (any, error) {
	handler, exists := GetMethodRegistry()[method]
	if !exists {
		return nil, fmt.Errorf("%w: %s", errMethodNotFound, method)
	}
	if httpOnlyMethods[method] {
		return handler(params)
	}
	// a nil RawMessage would be sent as JSON null, which handlers read as
	// present-but-invalid instead of missing
	var forwarded any
	if len(params) > 0 {
		forwarded = params
	}
	result, err := daemon.CallWithTimeout(context.Background(), method, forwarded, rpcTimeout(method))
	if err != nil {
		return nil, err
	}
	return result, nil
}

// streamFromDaemon runs a cli.* streaming method and hands each payload to
// onChunk (decoded bytes) or onProgress (text). It returns the envelope error, if any.
func streamFromDaemon(ctx context.Context, method string, params any, onChunk func([]byte) error, onProgress func(string)) error {
	raw, err := daemon.CallStream(ctx, method, params, func(payload json.RawMessage) error {
		var msg struct {
			Data     *string `json:"data"`
			Progress *string `json:"progress"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return err
		}
		switch {
		case msg.Data != nil:
			bytes, err := base64.StdEncoding.DecodeString(*msg.Data)
			if err != nil {
				return err
			}
			return onChunk(bytes)
		case msg.Progress != nil:
			if onProgress != nil {
				onProgress(*msg.Progress)
			}
			return nil
		default:
			// ndjson line (log entry): forward with its newline
			return onChunk(append([]byte(payload), '\n'))
		}
	})
	if err != nil {
		return err
	}
	var env struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if env.Status == statusError {
		return errors.New(env.Error)
	}
	return nil
}

const statusError = "error"

// writeFlusher writes and flushes each chunk so it reaches the client immediately.
func writeFlusher(w http.ResponseWriter) func([]byte) error {
	fw := flushWriter{w: w}
	return func(b []byte) error {
		_, err := fw.Write(b)
		return err
	}
}

// ResolvedDevice is the result of device.resolve.
type ResolvedDevice struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
}

// handleDeviceResolve answers which device an id (or auto-select) maps to,
// without starting anything on it. The HTTP front uses it to validate and pin
// a device when minting stream session tickets.
func handleDeviceResolve(params json.RawMessage) (any, error) {
	var req struct {
		DeviceID string `json:"deviceId"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}
	}
	targetDevice, err := commands.FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return nil, fmt.Errorf("error finding device: %w", err)
	}
	return ResolvedDevice{ID: targetDevice.ID(), Platform: targetDevice.Platform(), Type: targetDevice.DeviceType()}, nil
}

// resolveDevice asks the daemon which device an id refers to.
func resolveDevice(deviceID string) (ResolvedDevice, error) {
	raw, err := daemon.Call("device.resolve", map[string]string{"deviceId": deviceID})
	if err != nil {
		return ResolvedDevice{}, err
	}
	var target ResolvedDevice
	if err := json.Unmarshal(raw, &target); err != nil {
		return ResolvedDevice{}, err
	}
	return target, nil
}

func resolveDeviceID(deviceID string) (string, error) {
	target, err := resolveDevice(deviceID)
	if err != nil {
		return "", err
	}
	return target.ID, nil
}
