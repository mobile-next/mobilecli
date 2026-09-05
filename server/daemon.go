package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
)

// httpOnlyMethods are served by the HTTP front itself (session tickets, its own
// lifecycle) and must never reach the daemon.
var httpOnlyMethods = map[string]bool{
	"device.logs":          true,
	"device.screencapture": true,
	"server.info":          true,
	"server.shutdown":      true,
}

// DaemonDispatch is the daemon's method router. CLI methods (cli.*) return the
// full CommandResponse envelope as the result, so the CLI prints exactly what
// it printed when it ran in-process. Everything else goes through the JSON-RPC
// registry shared with the HTTP server.
func DaemonDispatch(ctx context.Context, method string, params json.RawMessage, notify func(any) error) (any, error) {
	if handler, ok := commands.CLIRegistry()[method]; ok {
		return handler(params), nil
	}
	if handler, ok := cliStreamRegistry()[method]; ok {
		return handler(ctx, params, notify)
	}
	if httpOnlyMethods[method] {
		return nil, fmt.Errorf("method '%s' not found", method)
	}
	return Execute(method, params)
}

// DaemonBusy reports work that must keep the daemon alive past its idle timeout.
func DaemonBusy() bool {
	return recorder.active() || commands.HoldingLocationOverride()
}

type cliStreamHandler func(ctx context.Context, params json.RawMessage, notify func(any) error) (any, error)

// cliStreamRegistry holds CLI methods that emit stream.data notifications.
func cliStreamRegistry() map[string]cliStreamHandler {
	return map[string]cliStreamHandler{
		"cli.device.logs":   streamLogs,
		"cli.screencapture": streamScreenCapture,
		"cli.screenrecord":  streamScreenRecord,
	}
}
