package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatchRPCForwardsDeviceMethodsToDaemon(t *testing.T) {
	result, err := dispatchRPC("devices.list", json.RawMessage(`{}`))
	require.NoError(t, err)

	data, err := json.Marshal(result)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"devices"`)
}

func TestDispatchRPCReturnsDaemonErrorMessage(t *testing.T) {
	_, err := dispatchRPC("device.io.tap", json.RawMessage(`{"deviceId":"__nope__","x":1,"y":2}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "__nope__")
}

func TestDispatchRPCServesHTTPOnlyMethodsLocally(t *testing.T) {
	result, err := dispatchRPC("server.info", nil)
	require.NoError(t, err)
	info, ok := result.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, "mobilecli", info["name"])
}

func TestDispatchRPCUnknownMethodIsNotForwarded(t *testing.T) {
	_, err := dispatchRPC("nope.nope", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, errMethodNotFound)
}

func TestRPCTimeoutIsExtendedForSlowMethods(t *testing.T) {
	assert.Greater(t, rpcTimeout("device.apps.install"), rpcTimeout("device.io.tap"))
}

func TestDispatchRPCForwardsMissingParamsAsMissing(t *testing.T) {
	_, err := dispatchRPC("device.info", nil)
	require.Error(t, err)
	assert.Equal(t, "'params' is required with fields: deviceId", err.Error())
}
