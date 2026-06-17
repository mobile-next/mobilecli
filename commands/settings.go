package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

// ApplySettingsRequest applies device-level settings to a device. Pointer
// fields distinguish "not provided" from a zero value, so only the settings
// explicitly set are touched (PATCH semantics).
type ApplySettingsRequest struct {
	DeviceID   string  `json:"deviceId"`
	Animations *string `json:"animations,omitempty"` // "on" or "off"
}

// ApplySettingsCommand applies the provided device settings. Settings that a
// platform cannot honor are skipped with a debug log and never fail the call.
func ApplySettingsCommand(req ApplySettingsRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if req.Animations != nil {
		err = applyAnimations(device, *req.Animations)
		if err != nil {
			return NewErrorResponse(err)
		}
	}

	return NewSuccessResponse(OK)
}

func applyAnimations(device devices.ControllableDevice, animations string) error {
	if animations != "on" && animations != "off" {
		return fmt.Errorf("invalid value for animations '%s', must be 'on' or 'off'", animations)
	}

	configurable, ok := device.(devices.AnimationConfigurable)
	if !ok {
		utils.Verbose("animations not supported on %s (%s), skipping", device.ID(), device.Platform())
		return nil
	}

	err := configurable.SetAnimationsEnabled(animations == "on")
	if err != nil {
		return fmt.Errorf("failed to apply animations setting: %v", err)
	}

	return nil
}
