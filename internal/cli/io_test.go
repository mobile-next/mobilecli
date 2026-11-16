package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoordinatesAreParsedAsAPoint(t *testing.T) {
	target, err := parseScreenPoint("150,300")

	require.NoError(t, err)
	assert.Equal(t, screenPoint{X: 150, Y: 300}, target)
}

func TestSpacesAroundCoordinatesAreIgnored(t *testing.T) {
	target, err := parseScreenPoint(" 150 , 300 ")

	require.NoError(t, err)
	assert.Equal(t, screenPoint{X: 150, Y: 300}, target)
}

func TestAnElementRefIsPassedThroughForTheDeviceToResolve(t *testing.T) {
	target, err := parseScreenPoint("@e45")

	require.NoError(t, err)
	assert.Equal(t, screenPoint{Ref: "@e45"}, target)
}

func TestSomethingThatIsNeitherCoordinatesNorARefIsRejected(t *testing.T) {
	for _, arg := range []string{"150", "150,300,450", "left,300", ""} {
		_, err := parseScreenPoint(arg)

		assert.Error(t, err, "expected %q to be rejected", arg)
	}
}
