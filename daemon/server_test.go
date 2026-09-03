package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echoDispatcher answers "echo" with its params, "fail" with an error,
// "stream" with three notifications, and "block" by waiting for ctx.
func echoDispatcher(blocked chan struct{}) Dispatcher {
	return func(ctx context.Context, method string, params json.RawMessage, notify func(any) error) (any, error) {
		switch method {
		case "echo":
			return params, nil
		case "fail":
			return nil, errors.New("device not found")
		case "stream":
			for i := 0; i < 3; i++ {
				if err := notify(map[string]int{"n": i}); err != nil {
					return nil, err
				}
			}
			return map[string]string{"done": "yes"}, nil
		case "block":
			<-ctx.Done()
			close(blocked)
			return nil, ctx.Err()
		}
		return nil, errors.New("unknown method " + method)
	}
}

type runningDaemon struct {
	paths   FilePaths
	done    chan error
	blocked chan struct{}
	cancel  context.CancelFunc
	result  error
	stopped bool
}

// waitStopped returns Run's result once, caching it for later callers.
func (rd *runningDaemon) waitStopped(timeout time.Duration) (error, bool) {
	if rd.stopped {
		return rd.result, true
	}
	select {
	case err := <-rd.done:
		rd.result, rd.stopped = err, true
		return err, true
	case <-time.After(timeout):
		return nil, false
	}
}

func startTestDaemon(t *testing.T, idle time.Duration, busy func() bool) *runningDaemon {
	t.Helper()
	t.Setenv(HomeEnv, shortTempDir(t))
	paths, err := Paths()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	rd := &runningDaemon{paths: paths, done: make(chan error, 1), blocked: make(chan struct{}), cancel: cancel}
	go func() {
		rd.done <- Run(ctx, Options{
			Paths:       paths,
			Version:     "test-1",
			IdleTimeout: idle,
			Dispatch:    echoDispatcher(rd.blocked),
			Busy:        busy,
		})
	}()
	require.NoError(t, WaitForSocket(paths.Socket, 5*time.Second))
	t.Cleanup(func() {
		cancel()
		if _, ok := rd.waitStopped(5 * time.Second); !ok {
			t.Error("daemon did not stop")
		}
	})
	return rd
}

// shortTempDir avoids macOS's long $TMPDIR blowing the socket path limit.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "mcli")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestCallRoundTripsResult(t *testing.T) {
	startTestDaemon(t, 0, nil)

	result, err := Call("echo", map[string]string{"hello": "world"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"hello":"world"}`, string(result))
}

func TestCallSurfacesDispatcherErrorAsRPCError(t *testing.T) {
	startTestDaemon(t, 0, nil)

	_, err := Call("fail", nil)
	var rpcErr *RPCError
	require.ErrorAs(t, err, &rpcErr)
	assert.Equal(t, -32000, rpcErr.Code)
	assert.Equal(t, "device not found", rpcErr.Error())
}

func TestCallStreamDeliversNotificationsThenResult(t *testing.T) {
	startTestDaemon(t, 0, nil)

	var got []string
	result, err := CallStream(context.Background(), "stream", nil, func(p json.RawMessage) error {
		got = append(got, string(p))
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{`{"n":0}`, `{"n":1}`, `{"n":2}`}, got)
	assert.JSONEq(t, `{"done":"yes"}`, string(result))
}

func TestClientDisconnectCancelsHandlerContext(t *testing.T) {
	rd := startTestDaemon(t, 0, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	_, err := CallStream(ctx, "block", nil, nil)
	require.Error(t, err)

	select {
	case <-rd.blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("handler context was not cancelled after client went away")
	}
}

func TestDaemonStatusReportsVersionAndPid(t *testing.T) {
	startTestDaemon(t, 0, nil)

	st, err := Status()
	require.NoError(t, err)
	assert.Equal(t, "test-1", st.Version)
	assert.Equal(t, os.Getpid(), st.PID)
}

func TestShutdownMethodStopsRunAndRemovesFiles(t *testing.T) {
	rd := startTestDaemon(t, 0, nil)
	_, err := os.Stat(rd.paths.Pid)
	require.NoError(t, err, "pid file should exist while running")

	require.NoError(t, Shutdown())

	err, ok := rd.waitStopped(5 * time.Second)
	require.True(t, ok, "Run did not return after daemon.shutdown")
	require.NoError(t, err)
	assert.NoFileExists(t, rd.paths.Pid)
	assert.NoFileExists(t, rd.paths.Socket)
}

func TestIdleTimeoutStopsDaemon(t *testing.T) {
	rd := startTestDaemon(t, 200*time.Millisecond, nil)

	err, ok := rd.waitStopped(5 * time.Second)
	require.True(t, ok, "daemon did not stop on idle")
	require.NoError(t, err)
}

func TestBusyBlocksIdleTimeout(t *testing.T) {
	rd := startTestDaemon(t, 200*time.Millisecond, func() bool { return true })

	_, stopped := rd.waitStopped(1 * time.Second)
	assert.False(t, stopped, "daemon stopped while busy")
}

func TestCallWhenNotRunningReturnsErrNotRunning(t *testing.T) {
	t.Setenv(HomeEnv, shortTempDir(t))

	_, err := Call("echo", nil)
	assert.ErrorIs(t, err, ErrNotRunning)
}

func TestRunRefusesToStartWhenAnotherDaemonIsListening(t *testing.T) {
	rd := startTestDaemon(t, 0, nil)

	err := Run(context.Background(), Options{Paths: rd.paths, Version: "test-2"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
	st, err := Status()
	require.NoError(t, err)
	assert.Equal(t, "test-1", st.Version, "the original daemon must be untouched")
}
