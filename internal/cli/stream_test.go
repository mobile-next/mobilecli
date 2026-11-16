package cli

import (
	"bytes"
	"encoding/json"
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
