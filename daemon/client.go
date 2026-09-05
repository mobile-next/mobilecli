package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// ErrNotRunning is returned when no daemon is listening on the socket.
var ErrNotRunning = errors.New("daemon is not running")

const (
	dialTimeout = 2 * time.Second
	// DefaultCallTimeout bounds a plain request/response call.
	DefaultCallTimeout = 60 * time.Second
	// NoTimeout disables the read deadline; cancel via context instead.
	NoTimeout = 0
)

// Call sends one request and returns the raw result. A daemon-side failure is
// returned as *RPCError; a connection failure wraps ErrNotRunning.
func Call(method string, params any) (json.RawMessage, error) {
	return CallWithTimeout(context.Background(), method, params, DefaultCallTimeout)
}

// CallWithTimeout is Call with an explicit read deadline (0 means none).
func CallWithTimeout(ctx context.Context, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	return call(ctx, method, params, timeout, nil)
}

// cancelGrace bounds how long a cancelled call waits for the daemon's final
// result (a screen recording still has to be finalized and pulled).
const cancelGrace = 60 * time.Second

// CallStream is Call for streaming methods: onData receives every stream.data
// notification payload before the final result arrives. Cancelling ctx cancels
// the handler on the daemon side and waits for its final result.
func CallStream(ctx context.Context, method string, params any, onData func(json.RawMessage) error) (json.RawMessage, error) {
	return call(ctx, method, params, 0, onData)
}

func call(ctx context.Context, method string, params any, timeout time.Duration, onData func(json.RawMessage) error) (json.RawMessage, error) {
	paths, err := Paths()
	if err != nil {
		return nil, err
	}

	conn, err := dial(paths.Socket)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	// cancelling ctx asks the daemon to cancel the handler but keeps the
	// connection open so the handler's final result (e.g. a finalized
	// recording) still arrives. the deadline is the safety net if it never does.
	stop := context.AfterFunc(ctx, func() {
		_ = writeFrame(conn, request{JSONRPC: jsonrpcVersion, Method: CancelMethod})
		_ = conn.SetReadDeadline(time.Now().Add(cancelGrace))
	})
	defer stop()

	if err := sendRequest(conn, method, params); err != nil {
		return nil, err
	}
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
	}

	result, err := readResponse(bufio.NewReader(conn), onData)
	if err != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return result, err
}

func sendRequest(conn net.Conn, method string, params any) error {
	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		rawParams = data
	}
	if err := writeFrame(conn, request{JSONRPC: jsonrpcVersion, ID: 1, Method: method, Params: rawParams}); err != nil {
		return fmt.Errorf("write to daemon: %w", err)
	}
	return nil
}

// readResponse consumes stream notifications until the final response.
func readResponse(reader *bufio.Reader, onData func(json.RawMessage) error) (json.RawMessage, error) {
	for {
		line, err := readFrame(reader)
		if err != nil {
			return nil, fmt.Errorf("read from daemon: %w", err)
		}

		var resp response
		if err := json.Unmarshal(line, &resp); err != nil {
			return nil, fmt.Errorf("invalid daemon response: %w", err)
		}

		// notifications carry a method; error replies to unparseable requests carry no id
		if resp.Method != "" {
			if resp.Method == StreamMethod && onData != nil {
				if err := onData(resp.Params); err != nil {
					return nil, err
				}
			}
			continue
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func dial(socket string) (net.Conn, error) {
	conn, err := net.DialTimeout("unix", socket, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotRunning, err)
	}
	return conn, nil
}

// Status queries a running daemon.
func Status() (*StatusResult, error) {
	raw, err := Call(methodStatus, nil)
	if err != nil {
		return nil, err
	}
	var st StatusResult
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// Shutdown asks a running daemon to exit and waits for it to go away.
func Shutdown() error {
	paths, err := Paths()
	if err != nil {
		return err
	}
	info, _ := readPidFile(paths.Pid)

	if _, err := Call(methodShutdown, nil); err != nil {
		return err
	}
	if err := waitForSocketGone(paths.Socket, shutdownGrace); err != nil {
		return err
	}
	if info.PID == 0 || info.PID == os.Getpid() {
		return nil
	}
	return pollUntil(shutdownGrace, func() bool { return !processExists(info.PID) }, "daemon process did not exit")
}

// WaitForSocket polls until something accepts connections on the socket.
func WaitForSocket(socket string, timeout time.Duration) error {
	return pollUntil(timeout, func() bool {
		conn, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, "daemon did not start listening")
}

func waitForSocketGone(socket string, timeout time.Duration) error {
	return pollUntil(timeout, func() bool {
		conn, err := net.DialTimeout("unix", socket, 500*time.Millisecond)
		if err != nil {
			return true
		}
		_ = conn.Close()
		return false
	}, "daemon did not stop")
}

func pollUntil(timeout time.Duration, cond func() bool, failMsg string) error {
	deadline := time.Now().Add(timeout)
	wait := 50 * time.Millisecond
	for {
		if cond() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s within %s", failMsg, timeout)
		}
		time.Sleep(wait)
		if wait < 500*time.Millisecond {
			wait *= 2
		}
	}
}
