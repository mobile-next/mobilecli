package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveScreenshotPathUsesExplicitFile(t *testing.T) {
	got, err := resolveScreenshotPath("/tmp/x.png", "dev:1", "png")
	require.NoError(t, err)
	assert.Equal(t, "/tmp/x.png", got)
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
