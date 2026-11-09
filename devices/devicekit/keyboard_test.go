package devicekit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// agentAnswering is an agent that gives the same JSON-RPC body to every request.
func agentAnswering(t *testing.T, body string) *DeviceKitClient {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(agent.Close)
	return NewDeviceKitClient(agent.URL)
}

func TestIsKeyboardVisibleReadsTheAgentAnswer(t *testing.T) {
	client := agentAnswering(t, `{"jsonrpc":"2.0","id":1,"result":{"visible":true}}`)

	visible, err := client.IsKeyboardVisible()
	if err != nil {
		t.Fatalf("IsKeyboardVisible: %v", err)
	}
	if !visible {
		t.Error("visible = false, want true")
	}
}

func TestAnOldAgentGetsTheInstallInstruction(t *testing.T) {
	client := agentAnswering(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`)

	_, err := client.IsKeyboardVisible()
	if err == nil || !strings.Contains(err.Error(), "agent install") {
		t.Errorf("error = %v, want one that says to run agent install", err)
	}
}

func TestOtherAgentErrorsPassThrough(t *testing.T) {
	client := agentAnswering(t, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"No foreground app found"}}`)

	_, err := client.IsKeyboardVisible()
	if err == nil || !strings.Contains(err.Error(), "No foreground app found") {
		t.Errorf("error = %v, want the agent's own message", err)
	}
}
