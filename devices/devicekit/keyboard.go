package devicekit

import (
	"encoding/json"
	"errors"
	"fmt"
)

const rpcMethodNotFound = -32601

type keyboardStatusResult struct {
	Visible bool `json:"visible"`
}

// IsKeyboardVisible reports whether the software keyboard is showing in the
// foreground app.
func (c *DeviceKitClient) IsKeyboardVisible() (bool, error) {
	raw, err := c.CallRPC("device.io.keyboard.status", nil)
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) && rpcErr.Code == rpcMethodNotFound {
		return false, errors.New("the agent on this device does not support keyboard commands, run `mobilecli agent install --force`")
	}
	if err != nil {
		return false, err
	}
	var result keyboardStatusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return false, fmt.Errorf("failed to parse keyboard status response: %w", err)
	}
	return result.Visible, nil
}
