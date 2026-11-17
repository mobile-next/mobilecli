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

func TestResolveScreenRecordPathNamesTheFileAfterTheDevice(t *testing.T) {
	dir := t.TempDir()

	got, release, err := ResolveScreenRecordPath(dir+string(filepath.Separator), "dev:1")
	require.NoError(t, err)
	defer release()

	assert.Equal(t, dir, filepath.Dir(got))
	base := filepath.Base(got)
	assert.True(t, strings.HasPrefix(base, "screenrecord-dev_1-"), base)
	assert.True(t, strings.HasSuffix(base, ".mp4"), base)
}

func TestTwoRecordingsInProgressNeverShareAGeneratedName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rec.mp4")

	first, releaseFirst := reserveRecordingPath(path)
	defer releaseFirst()
	second, releaseSecond := reserveRecordingPath(path)
	defer releaseSecond()

	assert.Equal(t, path, first)
	assert.Equal(t, filepath.Join(filepath.Dir(path), "rec-1.mp4"), second)
}

func TestAGeneratedRecordingNameIsFreeAgainAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rec.mp4")
	_, release := reserveRecordingPath(path)
	release()

	again, releaseAgain := reserveRecordingPath(path)
	defer releaseAgain()

	assert.Equal(t, path, again)
}

func TestAnExplicitRecordingPathIsNotReserved(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "mine.mp4")

	first, releaseFirst, err := ResolveScreenRecordPath(explicit, "dev")
	require.NoError(t, err)
	defer releaseFirst()
	second, releaseSecond, err := ResolveScreenRecordPath(explicit, "dev")
	require.NoError(t, err)
	defer releaseSecond()

	assert.Equal(t, explicit, first)
	assert.Equal(t, explicit, second)
}

func TestRecordingsThroughASymlinkAndItsTargetNeverShareAGeneratedName(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	first, releaseFirst := reserveRecordingPath(filepath.Join(target, "rec.mp4"))
	defer releaseFirst()
	second, releaseSecond := reserveRecordingPath(filepath.Join(link, "rec.mp4"))
	defer releaseSecond()

	assert.Equal(t, filepath.Join(target, "rec.mp4"), first)
	assert.Equal(t, filepath.Join(link, "rec-1.mp4"), second)
}
