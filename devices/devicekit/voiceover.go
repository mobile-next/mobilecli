package devicekit

import (
	"encoding/json"
	"fmt"
	"time"
)

// voiceOverSetTimeout covers the agent's own wait for VoiceOver to start or stop:
// disabling takes ~5s even when it is already off, and a slow start fails after ~15s
// with an error worth passing on rather than our own timeout.
const voiceOverSetTimeout = 30 * time.Second

type voiceOverState struct {
	Enabled bool `json:"enabled"`
}

type voiceOverOutput struct {
	Utterance string `json:"utterance"`
}

// GetVoiceOver reports whether VoiceOver is running (iOS 27+)
func (c *DeviceKitClient) GetVoiceOver() (bool, error) {
	result, err := c.CallRPC("device.voiceover.get", nil)
	if err != nil {
		return false, err
	}

	var state voiceOverState
	if err := json.Unmarshal(result, &state); err != nil {
		return false, fmt.Errorf("failed to parse voiceover state: %w", err)
	}

	return state.Enabled, nil
}

// SetVoiceOver turns VoiceOver on or off (iOS 27+). VoiceOver stays in that
// state after the agent exits.
func (c *DeviceKitClient) SetVoiceOver(enabled bool) error {
	_, err := c.CallRPCWithTimeout("device.voiceover.set", map[string]bool{"enabled": enabled}, voiceOverSetTimeout)
	return err
}

// VoiceOverSpeech returns what VoiceOver speaks for the element it is focused on
func (c *DeviceKitClient) VoiceOverSpeech() (string, error) {
	return c.voiceOverUtterance("device.voiceover.speech", nil)
}

// MoveVoiceOver moves the VoiceOver cursor ("next", "previous", "in" or "out")
// and returns what VoiceOver speaks for the element it lands on
func (c *DeviceKitClient) MoveVoiceOver(direction string) (string, error) {
	return c.voiceOverUtterance("device.voiceover.move", map[string]string{"direction": direction})
}

func (c *DeviceKitClient) voiceOverUtterance(method string, params any) (string, error) {
	result, err := c.CallRPC(method, params)
	if err != nil {
		return "", err
	}

	var output voiceOverOutput
	if err := json.Unmarshal(result, &output); err != nil {
		return "", fmt.Errorf("failed to parse voiceover speech: %w", err)
	}

	return output.Utterance, nil
}
