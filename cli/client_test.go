package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mobile-next/mobilecli/daemon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvelopeErrorReturnsNilForOkStatus(t *testing.T) {
	assert.NoError(t, envelopeError(json.RawMessage(`{"status":"ok","data":{"x":1}}`)))
}

func TestEnvelopeErrorReturnsTheErrorMessageForErrorStatus(t *testing.T) {
	err := envelopeError(json.RawMessage(`{"status":"error","error":"device not found"}`))
	require.Error(t, err)
	assert.Equal(t, "device not found", err.Error())
}

func TestEnvelopeErrorReturnsTheDataMessageForFailStatus(t *testing.T) {
	err := envelopeError(json.RawMessage(`{"status":"fail","data":{"message":"Agent is not installed on the device"}}`))
	require.Error(t, err)
	assert.Equal(t, "Agent is not installed on the device", err.Error())
}

func TestIndentedJSONPreservesKeyOrderOfRawResult(t *testing.T) {
	out, err := indentedJSON(json.RawMessage(`{"status":"ok","data":{"b":1,"a":2}}`))
	require.NoError(t, err)
	assert.Equal(t, "{\n  \"status\": \"ok\",\n  \"data\": {\n    \"b\": 1,\n    \"a\": 2\n  }\n}", out)
}

func TestDaemonSpawnArgsForwardVerboseAndInsecureStorage(t *testing.T) {
	verbose, insecureStorage = true, true
	t.Cleanup(func() { verbose, insecureStorage = false, false })

	args := daemonSpawnArgs()

	assert.Equal(t, []string{"daemon", "start", "--verbose", "--insecure-storage"}, args)
}

func TestTransportErrorBecomesErrorEnvelope(t *testing.T) {
	env := errorEnvelope(errors.New("boom"))
	assert.Equal(t, "error", env.Status)
	assert.Equal(t, "boom", env.Error)

	rpc := errorEnvelope(&daemon.RPCError{Code: -32000, Message: "Server error", Data: "detail"})
	assert.Equal(t, "detail", rpc.Error)
}

func TestAbsLocalPathKeepsStdoutDashAndEmpty(t *testing.T) {
	assert.Equal(t, "-", absLocalPath("-"))
	assert.Equal(t, "", absLocalPath(""))
}

func TestAbsLocalPathResolvesRelativeAgainstCwd(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cwd, "shot.png"), absLocalPath("shot.png"))
}
