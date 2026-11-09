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

type keyboardHideResult struct {
	Dismissed bool `json:"dismissed"`
}

// IsKeyboardVisible reports whether the software keyboard is showing in the
// foreground app.
func (c *DeviceKitClient) IsKeyboardVisible() (bool, error) {
	var result keyboardStatusResult
	err := c.callKeyboardMethod("device.io.keyboard.status", &result)
	return result.Visible, err
}

// HideKeyboard asks the agent to dismiss the keyboard. It returns false when
// no keyboard was showing.
func (c *DeviceKitClient) HideKeyboard() (bool, error) {
	var result keyboardHideResult
	err := c.callKeyboardMethod("device.io.keyboard.hide", &result)
	return result.Dismissed, err
}

func (c *DeviceKitClient) callKeyboardMethod(method string, result any) error {
	raw, err := c.CallRPC(method, nil)
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) && rpcErr.Code == rpcMethodNotFound {
		return errors.New("the agent on this device does not support keyboard commands, run `mobilecli agent install --force`")
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return fmt.Errorf("failed to parse %s response: %w", method, err)
	}
	return nil
}
