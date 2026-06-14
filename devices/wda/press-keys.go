package wda

// KeyCombo represents a single key press with optional modifier keys
type KeyCombo struct {
	Key       string   `json:"key"`
	Modifiers []string `json:"modifiers"`
}

func (c *WdaClient) PressKeys(combos []KeyCombo) error {
	params := map[string]any{
		"keys": combos,
	}

	_, err := c.CallRPC("device.io.keys", params)
	return err
}
