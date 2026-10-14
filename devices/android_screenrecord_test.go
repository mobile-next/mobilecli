package devices

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

const recordingPath = "/sdcard/mobilecli-rec-123.mp4"

func screenRecordPatternMatches(commandLine string) bool {
	return regexp.MustCompile(screenRecordProcessPattern(recordingPath)).MatchString(commandLine)
}

func TestScreenRecordPatternMatchesTheRecordingProcess(t *testing.T) {
	assert.True(t, screenRecordPatternMatches("screenrecord --time-limit 5 "+recordingPath))
	assert.True(t, screenRecordPatternMatches("screenrecord "+recordingPath))
}

func TestScreenRecordPatternDoesNotMatchThePgrepShellItself(t *testing.T) {
	pgrepCommandLine := "sh -c pgrep -f '" + screenRecordProcessPattern(recordingPath) + "'"
	assert.False(t, screenRecordPatternMatches(pgrepCommandLine))
}

func TestScreenRecordPatternDoesNotMatchAnotherRecording(t *testing.T) {
	assert.False(t, screenRecordPatternMatches("screenrecord /sdcard/mobilecli-rec-456.mp4"))
	assert.False(t, screenRecordPatternMatches("screenrecord /sdcard/mobilecli-rec-123xmp4"))
}
