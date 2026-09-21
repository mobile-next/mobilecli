package devices

import (
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStagingPathForIsASiblingOfTheDestination(t *testing.T) {
	staging, err := stagingPathFor("/data/local/tmp/mobilecli.dex")

	require.NoError(t, err)
	assert.Equal(t, "/data/local/tmp", path.Dir(staging), "rename is only atomic within one directory")
	assert.True(t, strings.HasPrefix(path.Base(staging), "mobilecli.dex."))
	assert.True(t, strings.HasSuffix(staging, ".tmp"))
}

func TestStagingPathForNeverRepeatsForTheSameDestination(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		staging, err := stagingPathFor("/data/local/tmp/mobilecli.so")
		require.NoError(t, err)
		require.False(t, seen[staging], "two installs would share %s", staging)
		seen[staging] = true
	}
}
