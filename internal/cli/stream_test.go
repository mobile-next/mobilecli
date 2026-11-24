package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteStreamPayloadDecodesBinaryChunksToStdout(t *testing.T) {
	var out, errOut bytes.Buffer
	require.NoError(t, writeStreamPayload(json.RawMessage(`{"data":"aGVsbG8="}`), &out, &errOut))
	assert.Equal(t, "hello", out.String())
	assert.Empty(t, errOut.String())
}

func TestWriteStreamPayloadSendsMessagesToStderrVerbatim(t *testing.T) {
	var out, errOut bytes.Buffer
	require.NoError(t, writeStreamPayload(json.RawMessage(`{"progress":"\rRecording 00:01"}`), &out, &errOut))
	assert.Empty(t, out.String())
	assert.Equal(t, "\rRecording 00:01", errOut.String())
}

func TestWriteStreamPayloadPrintsOtherObjectsAsNDJSONLines(t *testing.T) {
	var out, errOut bytes.Buffer
	require.NoError(t, writeStreamPayload(json.RawMessage(`{"level":"Error","message":"boom"}`), &out, &errOut))
	assert.Equal(t, "{\"level\":\"Error\",\"message\":\"boom\"}\n", out.String())
	assert.Empty(t, errOut.String())
}

func TestProgressLinesPutsEachCarriageReturnUpdateOnItsOwnLine(t *testing.T) {
	var out bytes.Buffer
	lines := progressLines{out: &out}

	for _, update := range []string{
		"Screen recording has started\n",
		"\rScreen recording for 00:01 seconds",
		"\rScreen recording for 00:02 seconds",
		"\n",
		"Saved video file to /tmp/a.mp4\n",
	} {
		written, err := lines.Write([]byte(update))
		require.NoError(t, err)
		assert.Equal(t, len(update), written)
	}

	assert.Equal(t, "Screen recording has started\n"+
		"Screen recording for 00:01 seconds\n"+
		"Screen recording for 00:02 seconds\n"+
		"Saved video file to /tmp/a.mp4\n", out.String())
}

func TestProgressLinesSplitsSeveralUpdatesInOneWrite(t *testing.T) {
	var out bytes.Buffer

	_, err := progressLines{out: &out}.Write([]byte("\nScreen recording ended\n12 frames, 2s\nSaved\n"))

	require.NoError(t, err)
	assert.Equal(t, "Screen recording ended\n12 frames, 2s\nSaved\n", out.String())
}

func TestProgressWriterSplitsUpdatesIntoLinesForARegularFile(t *testing.T) {
	regularFile, err := os.Create(filepath.Join(t.TempDir(), "stderr.txt"))
	require.NoError(t, err)
	defer func() { _ = regularFile.Close() }()

	assert.IsType(t, progressLines{}, progressWriter(regularFile))
}

func TestProgressWriterLeavesACharacterDeviceUntouched(t *testing.T) {
	// the null device is a character device, as a terminal is
	characterDevice, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)
	defer func() { _ = characterDevice.Close() }()

	assert.Same(t, characterDevice, progressWriter(characterDevice))
}
