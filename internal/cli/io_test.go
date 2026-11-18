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

func TestADragIsParsedAsWhereItStartsAndWhereItEnds(t *testing.T) {
	cases := map[string][2]screenPoint{
		"10,10,50,50":     {{X: 10, Y: 10}, {X: 50, Y: 50}},
		"@e1,@e2":         {{Ref: "@e1"}, {Ref: "@e2"}},
		"@e1,300,400":     {{Ref: "@e1"}, {X: 300, Y: 400}},
		"300,400,@e2":     {{X: 300, Y: 400}, {Ref: "@e2"}},
		" 10 , 10 , @e2 ": {{X: 10, Y: 10}, {Ref: "@e2"}},
	}

	for arg, want := range cases {
		from, to, err := parseDragPoints(arg)

		require.NoError(t, err, "expected %q to parse", arg)
		assert.Equal(t, want, [2]screenPoint{from, to}, "for %q", arg)
	}
}

func TestADragThatDoesNotNameExactlyTwoPointsIsRejected(t *testing.T) {
	for _, arg := range []string{"", "@e1", "10,10", "10,10,50", "10,10,50,50,60", "@e1,@e2,@e3", "@e1,300", "left,10,50,50"} {
		_, _, err := parseDragPoints(arg)

		assert.Error(t, err, "expected %q to be rejected", arg)
	}
}
