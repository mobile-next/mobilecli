package devices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteStopNamesTheRecordingItStarted(t *testing.T) {
	assert.Equal(t, params{"recordingId": "abc"}, recordingStopParams("abc"))
}

func TestRemoteStopWithoutARecordingIDIsAPlainStop(t *testing.T) {
	assert.Equal(t, params{}, recordingStopParams(""))
}
