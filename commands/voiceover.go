package commands

import (
	"fmt"
	"strings"
)

type VoiceOverGetRequest struct {
	DeviceID string `json:"deviceId"`
}

type VoiceOverSetRequest struct {
	DeviceID string `json:"deviceId"`
	State    string `json:"state"`
}

type VoiceOverSpeechRequest struct {
	DeviceID string `json:"deviceId"`
}

type VoiceOverMoveRequest struct {
	DeviceID  string `json:"deviceId"`
	Direction string `json:"direction"`
}

type VoiceOverStateResult struct {
	Enabled bool `json:"enabled"`
}

type VoiceOverSpeechResult struct {
	Utterance string `json:"utterance"`
}

// voiceOverController is implemented by devices whose agent drives VoiceOver
// through XCUIDevice.voiceOverService: iOS 27+ simulators and real devices
type voiceOverController interface {
	GetVoiceOver() (bool, error)
	SetVoiceOver(enabled bool) error
	VoiceOverSpeech() (string, error)
	MoveVoiceOver(direction string) (string, error)
}

var voiceOverDirections = []string{"next", "previous", "in", "out"}

// parseVoiceOverState accepts "on" or "off"
func parseVoiceOverState(state string) (bool, error) {
	switch strings.ToLower(state) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	}
	return false, fmt.Errorf("invalid voiceover state '%s', must be 'on' or 'off'", state)
}

func parseVoiceOverDirection(direction string) (string, error) {
	lower := strings.ToLower(direction)
	for _, d := range voiceOverDirections {
		if lower == d {
			return d, nil
		}
	}
	return "", fmt.Errorf("invalid voiceover direction '%s', must be 'next', 'previous', 'in' or 'out'", direction)
}

func findVoiceOverDevice(deviceID string) (voiceOverController, string, error) {
	targetDevice, err := FindDeviceWithAgent(deviceID)
	if err != nil {
		return nil, "", err
	}

	controller, ok := targetDevice.(voiceOverController)
	if !ok {
		return nil, "", fmt.Errorf("device %s does not support voiceover, only iOS 27+ devices and simulators do", targetDevice.ID())
	}

	return controller, targetDevice.ID(), nil
}

// VoiceOverGetCommand reports whether VoiceOver is running
func VoiceOverGetCommand(req VoiceOverGetRequest) *CommandResponse {
	controller, id, err := findVoiceOverDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	enabled, err := controller.GetVoiceOver()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to get voiceover state on device %s: %v", id, err))
	}

	return NewSuccessResponse(VoiceOverStateResult{Enabled: enabled})
}

// VoiceOverSetCommand turns VoiceOver on or off. VoiceOver stays on after
// mobilecli exits, and while it is on a tap focuses an element instead of
// activating it.
func VoiceOverSetCommand(req VoiceOverSetRequest) *CommandResponse {
	enabled, err := parseVoiceOverState(req.State)
	if err != nil {
		return NewErrorResponse(err)
	}

	controller, id, err := findVoiceOverDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if err := controller.SetVoiceOver(enabled); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to set voiceover on device %s: %v", id, err))
	}

	return NewSuccessResponse(VoiceOverStateResult{Enabled: enabled})
}

// VoiceOverSpeechCommand returns what VoiceOver speaks for the focused element
func VoiceOverSpeechCommand(req VoiceOverSpeechRequest) *CommandResponse {
	controller, id, err := findVoiceOverDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	utterance, err := controller.VoiceOverSpeech()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to read voiceover speech on device %s: %v", id, err))
	}

	return NewSuccessResponse(VoiceOverSpeechResult{Utterance: utterance})
}

// VoiceOverMoveCommand moves the VoiceOver cursor and returns what it speaks
func VoiceOverMoveCommand(req VoiceOverMoveRequest) *CommandResponse {
	direction, err := parseVoiceOverDirection(req.Direction)
	if err != nil {
		return NewErrorResponse(err)
	}

	controller, id, err := findVoiceOverDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	utterance, err := controller.MoveVoiceOver(direction)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to move voiceover %s on device %s: %v", direction, id, err))
	}

	return NewSuccessResponse(VoiceOverSpeechResult{Utterance: utterance})
}
