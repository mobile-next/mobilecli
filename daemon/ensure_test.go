package daemon

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDaemonEnv        = "MOBILECLI_TEST_DAEMON"
	testDaemonVersionEnv = "MOBILECLI_TEST_DAEMON_VERSION"
)

// TestMain lets the test binary double as the daemon executable: when
// MOBILECLI_TEST_DAEMON=1 it runs a daemon with the echo dispatcher instead of
// the tests. EnsureRunning re-execs os.Executable() with these env vars set.
func TestMain(m *testing.M) {
	if os.Getenv(testDaemonEnv) == "1" {
		paths, err := Paths()
		if err != nil {
			os.Exit(2)
		}
		err = Run(context.Background(), Options{
			Paths:       paths,
			Version:     os.Getenv(testDaemonVersionEnv),
			IdleTimeout: time.Minute,
			Dispatch:    echoDispatcher(make(chan struct{})),
		})
		if err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// spawnArgs makes the re-exec'd test binary skip every test; TestMain does the work.
var spawnArgs = []string{"-test.run=^$"}

func setupSpawnEnv(t *testing.T, version string) {
	t.Helper()
	t.Setenv(HomeEnv, shortTempDir(t))
	t.Setenv(testDaemonEnv, "1")
	t.Setenv(testDaemonVersionEnv, version)
	t.Cleanup(func() { _ = Shutdown() })
}

func TestEnsureRunningSpawnsDaemonWhenNoneIsRunning(t *testing.T) {
	setupSpawnEnv(t, "v1")

	require.NoError(t, EnsureRunning("v1", spawnArgs))

	st, err := Status()
	require.NoError(t, err)
	assert.Equal(t, "v1", st.Version)
	assert.NotEqual(t, os.Getpid(), st.PID)
}

func TestEnsureRunningReusesDaemonWithSameVersion(t *testing.T) {
	setupSpawnEnv(t, "v1")
	require.NoError(t, EnsureRunning("v1", spawnArgs))
	first, err := Status()
	require.NoError(t, err)

	require.NoError(t, EnsureRunning("v1", spawnArgs))

	second, err := Status()
	require.NoError(t, err)
	assert.Equal(t, first.PID, second.PID)
}

func TestEnsureRunningRestartsDaemonOnVersionMismatch(t *testing.T) {
	setupSpawnEnv(t, "v1")
	require.NoError(t, EnsureRunning("v1", spawnArgs))
	old, err := Status()
	require.NoError(t, err)

	// the newly spawned daemon reports v2, and the client is v2
	t.Setenv(testDaemonVersionEnv, "v2")
	require.NoError(t, EnsureRunning("v2", spawnArgs))

	st, err := Status()
	require.NoError(t, err)
	assert.Equal(t, "v2", st.Version)
	assert.NotEqual(t, old.PID, st.PID)
	assert.False(t, processExists(old.PID), "old daemon should be gone")
}

func TestEnsureRunningCleansStalePidFile(t *testing.T) {
	setupSpawnEnv(t, "v1")
	paths, err := Paths()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(paths.Dir, 0o700))
	require.NoError(t, writePidFile(paths.Pid, pidInfo{PID: 2_000_000_000, Version: "v1"}))
	require.NoError(t, os.WriteFile(paths.Socket, []byte("not a socket"), 0o600))

	require.NoError(t, EnsureRunning("v1", spawnArgs))

	st, err := Status()
	require.NoError(t, err)
	assert.Equal(t, "v1", st.Version)
}

func TestEnsureRunningFailsWithLogTailWhenChildDies(t *testing.T) {
	setupSpawnEnv(t, "v1")
	t.Setenv(testDaemonEnv, "0") // child runs the (empty) test set and exits immediately

	err := EnsureRunning("v1", spawnArgs)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon.log")
}

func TestConcurrentEnsureRunningStartsExactlyOneDaemon(t *testing.T) {
	setupSpawnEnv(t, "v1")

	const n = 5
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() { errs <- EnsureRunning("v1", spawnArgs) }()
	}
	for i := 0; i < n; i++ {
		require.NoError(t, <-errs)
	}

	st, err := Status()
	require.NoError(t, err)
	paths, err := Paths()
	require.NoError(t, err)
	info, err := readPidFile(paths.Pid)
	require.NoError(t, err)
	assert.Equal(t, st.PID, info.PID, "pid file must belong to the daemon that owns the socket")

	// after shutdown nothing may be left listening: a second daemon would still answer
	require.NoError(t, Shutdown())
	_, err = Status()
	assert.ErrorIs(t, err, ErrNotRunning)
}
