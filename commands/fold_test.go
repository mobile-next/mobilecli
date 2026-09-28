package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamedFoldStatesMapToAngles(t *testing.T) {
	for state, expected := range map[string]float64{"folded": 0, "half-open": 90, "open": 180, "OPEN": 180} {
		angle, err := parseFoldAngle(state)

		require.NoError(t, err)
		assert.Equal(t, expected, angle, "state %q", state)
	}
}

func TestAFoldAngleCanBeGivenInDegrees(t *testing.T) {
	angle, err := parseFoldAngle("127.5")

	require.NoError(t, err)
	assert.Equal(t, 127.5, angle)
}

func TestFoldAnglesOutsideZeroTo180AreRejected(t *testing.T) {
	for _, state := range []string{"-1", "181", "sideways", ""} {
		_, err := parseFoldAngle(state)

		assert.Error(t, err, "expected %q to be rejected", state)
	}
}
