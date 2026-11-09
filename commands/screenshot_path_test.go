package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveScreenshotPathUsesExplicitFile(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "x.png")
	got, err := resolveScreenshotPath(explicit, "dev:1", "png")
	require.NoError(t, err)
	assert.Equal(t, explicit, got)
}

func TestResolveScreenshotPathGeneratesNameInsideDirectory(t *testing.T) {
	dir := t.TempDir()

	got, err := resolveScreenshotPath(dir+string(filepath.Separator), "dev:1", "jpeg")
	require.NoError(t, err)

	assert.Equal(t, dir, filepath.Dir(got))
	base := filepath.Base(got)
	assert.True(t, strings.HasPrefix(base, "screenshot-dev_1-"), base)
	assert.True(t, strings.HasSuffix(base, ".jpg"), base)
}

func TestResolveScreenshotPathEmptyMeansCurrentDirectory(t *testing.T) {
	got, err := resolveScreenshotPath("", "dev", "png")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(got))
	assert.True(t, strings.HasSuffix(got, ".png"))
}

func TestResolveScreenshotPathTreatsForwardSlashDirOnAnyOS(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveScreenshotPath(dir+"/", "dev", "png")
	require.NoError(t, err)
	assert.Equal(t, dir, filepath.Dir(got))
}

func TestUniquePathAppendsCounterWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "shot.png")
	require.NoError(t, os.WriteFile(base, nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "shot-1.png"), nil, 0o600))

	assert.Equal(t, filepath.Join(dir, "shot-2.png"), uniquePath(base))
	assert.Equal(t, filepath.Join(dir, "fresh.png"), uniquePath(filepath.Join(dir, "fresh.png")))
}
