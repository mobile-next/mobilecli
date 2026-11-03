package commands

import (
	"testing"

	"github.com/mobile-next/mobilecli/devices"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// every iOS device type, local or on a fleet, has to reach the agent's VoiceOver
var (
	_ voiceOverController = devices.SimulatorDevice{}
	_ voiceOverController = (*devices.IOSDevice)(nil)
	_ voiceOverController = (*devices.RemoteDevice)(nil)
)

func TestVoiceOverStatesParseToEnabled(t *testing.T) {
	for state, expected := range map[string]bool{"on": true, "off": false, "ON": true, "Off": false} {
		enabled, err := parseVoiceOverState(state)

		require.NoError(t, err)
		assert.Equal(t, expected, enabled, "state %q", state)
	}
}

func TestVoiceOverStatesOtherThanOnOrOffAreRejected(t *testing.T) {
	for _, state := range []string{"true", "enabled", "1", ""} {
		_, err := parseVoiceOverState(state)

		assert.Error(t, err, "expected %q to be rejected", state)
	}
}

func TestVoiceOverDirectionsAreNormalized(t *testing.T) {
	for input, expected := range map[string]string{"next": "next", "PREVIOUS": "previous", "In": "in", "out": "out"} {
		direction, err := parseVoiceOverDirection(input)

		require.NoError(t, err)
		assert.Equal(t, expected, direction, "direction %q", input)
	}
}

func TestUnknownVoiceOverDirectionsAreRejected(t *testing.T) {
	for _, direction := range []string{"forward", "up", ""} {
		_, err := parseVoiceOverDirection(direction)

		assert.Error(t, err, "expected %q to be rejected", direction)
	}
}

func TestVoiceOverSetRejectsInvalidStateBeforeLookingUpDevice(t *testing.T) {
	response := VoiceOverSetCommand(VoiceOverSetRequest{DeviceID: "__nope__", State: "maybe"})

	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Error, "invalid voiceover state")
}

func TestVoiceOverMoveRejectsInvalidDirectionBeforeLookingUpDevice(t *testing.T) {
	response := VoiceOverMoveCommand(VoiceOverMoveRequest{DeviceID: "__nope__", Direction: "up"})

	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Error, "invalid voiceover direction")
}
