package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentCommandsFailCleanlyForUnknownDevice(t *testing.T) {
	reg := CLIRegistry()
	for _, method := range []string{"cli.agent.status", "cli.agent.install", "cli.agent.uninstall"} {
		resp := reg[method](json.RawMessage(`{"deviceId":"__nope__"}`))
		assert.Equal(t, "error", resp.Status, method)
		assert.Contains(t, resp.Error, "__nope__", method)
	}
}

func TestAgentMatchesAppUsesSuffixOnIOSOnly(t *testing.T) {
	assert.True(t, agentMatchesApp("ios", "TEAM123.com.mobilenext.devicekit-iosUITests.xctrunner", iosRunnerBundleID))
	assert.False(t, agentMatchesApp("android", "x.com.example.app", "com.example.app"))
	assert.True(t, agentMatchesApp("android", "com.example.app", "com.example.app"))
}
