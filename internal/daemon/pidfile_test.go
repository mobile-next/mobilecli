package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndReadPidFileRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.pid")

	require.NoError(t, writePidFile(path, pidInfo{PID: 4242, Version: "1.2.3"}))

	info, err := readPidFile(path)
	require.NoError(t, err)
	assert.Equal(t, 4242, info.PID)
	assert.Equal(t, "1.2.3", info.Version)
}

func TestPidFileIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.pid")
	require.NoError(t, writePidFile(path, pidInfo{PID: 1}))

	st, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
}

func TestReadPidFileMissingReturnsError(t *testing.T) {
	_, err := readPidFile(filepath.Join(t.TempDir(), "nope.pid"))
	require.Error(t, err)
}

func TestProcessExistsForSelfAndNotForDeadPid(t *testing.T) {
	assert.True(t, processExists(os.Getpid()))
	// pid_max on linux is far below this; macOS caps at 99998
	assert.False(t, processExists(2_000_000_000))
}
