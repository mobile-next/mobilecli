package wda

import "time"

// KeyCombo represents a single key press with optional modifier keys
type KeyCombo struct {
	Key       string   `json:"key"`
	Modifiers []string `json:"modifiers"`
}

// per-key time budget added on top of the default RPC timeout, since all keys
// are pressed within a single RPC and the device types them sequentially
const perKeyTimeout = 2 * time.Second

// maxPressKeysTimeout keeps the request below the http client's own timeout
const maxPressKeysTimeout = 55 * time.Second

func (c *WdaClient) PressKeys(combos []KeyCombo) error {
	params := map[string]any{
		"keys": combos,
	}

	timeout := defaultRPCTimeout + time.Duration(len(combos))*perKeyTimeout
	if timeout > maxPressKeysTimeout {
		timeout = maxPressKeysTimeout
	}

	_, err := c.CallRPCWithTimeout("device.io.keys", params, timeout)
	return err
}
