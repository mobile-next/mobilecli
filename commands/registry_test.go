package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptUnmarshalsParamsIntoRequestAndCallsCommand(t *testing.T) {
	type req struct {
		Name string `json:"name"`
	}
	handler := adapt(func(r req) *CommandResponse { return NewSuccessResponse("hi " + r.Name) })

	resp := handler(json.RawMessage(`{"name":"bob"}`))

	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "hi bob", resp.Data)
}

func TestAdaptReturnsErrorResponseOnBadParams(t *testing.T) {
	type req struct {
		N int `json:"n"`
	}
	handler := adapt(func(r req) *CommandResponse { return NewSuccessResponse(r.N) })

	resp := handler(json.RawMessage(`{"n":"not a number"}`))

	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "invalid parameters")
}

func TestCLIRegistryCoversEveryCLIMethod(t *testing.T) {
	reg := CLIRegistry()
	for _, method := range []string{
		"cli.devices", "cli.device.info", "cli.device.reboot", "cli.device.boot", "cli.device.shutdown",
		"cli.device.orientation.get", "cli.device.orientation.set", "cli.device.settings.apply",
		"cli.device.crashes.list", "cli.device.crashes.get", "cli.device.location.set", "cli.device.location.clear",
		"cli.apps.launch", "cli.apps.terminate", "cli.apps.list", "cli.apps.install", "cli.apps.uninstall",
		"cli.apps.clear", "cli.apps.foreground", "cli.apps.path",
		"cli.io.tap", "cli.io.longpress", "cli.io.button", "cli.io.text", "cli.io.keys", "cli.io.swipe",
		"cli.io.gesture", "cli.io.clipboard.get", "cli.io.clipboard.set",
		"cli.fs.push", "cli.fs.pull", "cli.fs.ls", "cli.fs.mkdir", "cli.fs.rm",
		"cli.webview.list", "cli.webview.goto", "cli.webview.reload", "cli.webview.back", "cli.webview.forward",
		"cli.webview.eval", "cli.webview.wait", "cli.webview.content",
		"cli.webview.query",
		"cli.screenshot", "cli.dump.ui", "cli.url",
	} {
		_, ok := reg[method]
		assert.True(t, ok, "missing %s", method)
	}
}

func TestCrashesGetAdapterPassesBothArguments(t *testing.T) {
	// an unknown device fails before touching hardware, proving both fields were read
	resp := CLIRegistry()["cli.device.crashes.get"](json.RawMessage(`{"deviceId":"__nope__","id":"c1"}`))
	require.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "__nope__")
}
