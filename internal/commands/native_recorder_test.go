package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestANativeRecorderRefusesASecondRecordingOnTheSameDevice(t *testing.T) {
	release, err := claimNativeRecorder("device-a")
	require.NoError(t, err)
	defer release()

	_, err = claimNativeRecorder("device-a")

	assert.ErrorContains(t, err, "one recording at a time")
}

func TestANativeRecorderAllowsRecordingsOnDifferentDevices(t *testing.T) {
	release, err := claimNativeRecorder("device-a")
	require.NoError(t, err)
	defer release()

	otherRelease, err := claimNativeRecorder("device-b")

	require.NoError(t, err)
	otherRelease()
}

func TestANativeRecorderIsFreeAgainAfterRelease(t *testing.T) {
	release, err := claimNativeRecorder("device-a")
	require.NoError(t, err)
	release()

	again, err := claimNativeRecorder("device-a")

	require.NoError(t, err)
	again()
}
