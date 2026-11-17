package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirDefaultsToDotMobilecliInHome(t *testing.T) {
	t.Setenv("MOBILECLI_HOME", "")
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".mobilecli"), dir)
}

func TestDirHonorsMobilecliHomeEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MOBILECLI_HOME", tmp)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, tmp, dir)
}

func TestPathsLiveInsideDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MOBILECLI_HOME", tmp)

	p, err := Paths()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, "daemon.sock"), p.Socket)
	assert.Equal(t, filepath.Join(tmp, "daemon.pid"), p.Pid)
	assert.Equal(t, filepath.Join(tmp, "daemon.log"), p.Log)
}

func TestPathsRejectsSocketPathOverOSLimit(t *testing.T) {
	tooLong := filepath.Join(t.TempDir(), strings.Repeat("x", 120))
	t.Setenv("MOBILECLI_HOME", tooLong)

	_, err := Paths()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too long")
}
