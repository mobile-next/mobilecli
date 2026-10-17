package devices

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEmuSensorValueReadsTheReading(t *testing.T) {
	value, err := parseEmuSensorValue("acceleration = 0:9.77631:0.812349\nOK")

	require.NoError(t, err)
	assert.Equal(t, "0:9.77631:0.812349", value)
}

func TestParseEmuSensorValueRejectsAReplyWithoutAReading(t *testing.T) {
	for _, output := range []string{"", "OK", "acceleration = \nOK"} {
		_, err := parseEmuSensorValue(output)

		assert.Error(t, err, "expected %q to be rejected", output)
	}
}
