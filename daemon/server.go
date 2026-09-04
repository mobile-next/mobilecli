package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mobile-next/mobilecli/utils"
)

// Dispatcher executes one method. notify sends a stream notification to the
// caller before the final result; ctx is cancelled when the client disconnects.
type Dispatcher func(ctx context.Context, method string, params json.RawMessage, notify func(any) error) (any, error)

// Options configures Run.
type Options struct {
	Paths       FilePaths
	Version     string
	IdleTimeout time.Duration // 0 disables idle shutdown
	Dispatch    Dispatcher
	Busy        func() bool // optional: true blocks idle shutdown (e.g. recording in progress)
	OnShutdown  func()      // optional: runs after the listener closes and in-flight requests drain
}

// StatusResult is returned by daemon.status.
type StatusResult struct {
	Version   string    `json:"version"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
	Uptime    string    `json:"uptime"`
	Socket    string    `json:"socket"`
}

const (
	methodStatus   = "daemon.status"
	methodShutdown = "daemon.shutdown"

	errCodeServerError = -32000
	errCodeParseError  = -32700
	errCodeInvalid     = -32600

	idleTick          = 10 * time.Second
	shutdownGrace     = 10 * time.Second
	handlerTouchEvery = 30 * time.Second
)

type server struct {
	opts         Options
	startedAt    time.Time
	lastActivity atomic.Int64 // unix nanos
	inflight     atomic.Int32
	wg           sync.WaitGroup
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// Run listens on the unix socket and serves until ctx is cancelled, the idle
// timeout elapses, or a daemon.shutdown request arrives. It removes the socket
// and pid files on exit.
func Run(ctx context.Context, opts Options) error {
	if err := os.MkdirAll(opts.Paths.Dir, 0o700); err != nil {
		return fmt.Errorf("create daemon dir: %w", err)
	}
	// never steal the socket from a live daemon (two clients racing to spawn)
	if conn, err := net.DialTimeout("unix", opts.Paths.Socket, dialTimeout); err == nil {
		_ = conn.Close()
		return fmt.Errorf("daemon already running on %s", opts.Paths.Socket)
	}
	// a leftover socket file from a crashed daemon would make Listen fail
	_ = os.Remove(opts.Paths.Socket)

	ln, err := net.Listen("unix", opts.Paths.Socket)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", opts.Paths.Socket, err)
	}
	_ = os.Chmod(opts.Paths.Socket, 0o600)

	s := &server{opts: opts, startedAt: time.Now(), shutdown: make(chan struct{})}
	s.touch()

	if err := writePidFile(opts.Paths.Pid, pidInfo{PID: os.Getpid(), Version: opts.Version, StartedAt: s.startedAt}); err != nil {
		_ = ln.Close()
		return fmt.Errorf("write pid file: %w", err)
	}

	go s.acceptLoop(ln)
	if opts.IdleTimeout > 0 {
		go s.watchIdle()
	}

	select {
	case <-ctx.Done():
	case <-s.shutdown:
	}

	_ = ln.Close()
	s.waitInflight()
	if opts.OnShutdown != nil {
		opts.OnShutdown()
	}
	_ = os.Remove(opts.Paths.Socket)
	_ = os.Remove(opts.Paths.Pid)
	return nil
}

func (s *server) touch() { s.lastActivity.Store(time.Now().UnixNano()) }

func (s *server) requestShutdown() { s.shutdownOnce.Do(func() { close(s.shutdown) }) }

func (s *server) waitInflight() {
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
		utils.Info("daemon: timed out waiting for in-flight requests")
	}
}

func (s *server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *server) watchIdle() {
	tick := time.NewTicker(minDuration(idleTick, s.opts.IdleTimeout/2))
	defer tick.Stop()
	for {
		select {
		case <-s.shutdown:
			return
		case <-tick.C:
			if s.isIdle() {
				utils.Info("daemon: idle for %s, shutting down", s.opts.IdleTimeout)
				s.requestShutdown()
				return
			}
		}
	}
}

func (s *server) isIdle() bool {
	if s.inflight.Load() > 0 {
		return false
	}
	if s.opts.Busy != nil && s.opts.Busy() {
		return false
	}
	last := time.Unix(0, s.lastActivity.Load())
	return time.Since(last) >= s.opts.IdleTimeout
}

func minDuration(a, b time.Duration) time.Duration {
	if b > 0 && b < a {
		return b
	}
	return a
}

func (s *server) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	s.touch()
	defer s.touch()
	s.inflight.Add(1)
	defer s.inflight.Add(-1)

	line, err := readFrame(bufio.NewReader(conn))
	if err != nil {
		return
	}

	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		_ = writeFrame(conn, errorResponse(nil, errCodeParseError, "Parse error", err.Error()))
		return
	}
	if req.JSONRPC != jsonrpcVersion || req.Method == "" {
		_ = writeFrame(conn, errorResponse(req.ID, errCodeInvalid, "Invalid Request", "'jsonrpc' must be '2.0' and 'method' is required"))
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go cancelOnDisconnect(conn, cancel)

	var writeMu sync.Mutex
	notify := func(payload any) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		return writeFrame(conn, response{JSONRPC: jsonrpcVersion, Method: StreamMethod, Params: data})
	}

	result, err := s.dispatch(ctx, req, notify)

	writeMu.Lock()
	defer writeMu.Unlock()
	if err != nil {
		_ = writeFrame(conn, errorResponse(req.ID, errCodeServerError, "Server error", err.Error()))
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		_ = writeFrame(conn, errorResponse(req.ID, errCodeServerError, "Server error", err.Error()))
		return
	}
	_ = writeFrame(conn, response{JSONRPC: jsonrpcVersion, ID: req.ID, Result: data})
}

// cancelOnDisconnect blocks reading the (otherwise idle) connection; a read
// error means the client went away.
func cancelOnDisconnect(conn net.Conn, cancel context.CancelFunc) {
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			cancel()
			return
		}
	}
}

func (s *server) dispatch(ctx context.Context, req request, notify func(any) error) (any, error) {
	switch req.Method {
	case methodStatus:
		return StatusResult{
			Version:   s.opts.Version,
			PID:       os.Getpid(),
			StartedAt: s.startedAt,
			Uptime:    time.Since(s.startedAt).Round(time.Second).String(),
			Socket:    s.opts.Paths.Socket,
		}, nil
	case methodShutdown:
		// the response is written by the caller before Run tears the listener down
		go func() {
			time.Sleep(50 * time.Millisecond)
			s.requestShutdown()
		}()
		return map[string]string{"status": "ok"}, nil
	}
	if s.opts.Dispatch == nil {
		return nil, errors.New("no dispatcher configured")
	}
	return s.opts.Dispatch(ctx, req.Method, req.Params, notify)
}

func errorResponse(id any, code int, message string, data any) response {
	return response{JSONRPC: jsonrpcVersion, ID: id, Error: &RPCError{Code: code, Message: message, Data: data}}
}
