package devices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIOSDevicesRefuseToHideTheKeyboard(t *testing.T) {
	_, simulatorErr := SimulatorDevice{}.HideKeyboard()
	_, realDeviceErr := (&IOSDevice{}).HideKeyboard()

	assert.ErrorIs(t, simulatorErr, errHideKeyboardNotSupportedOnIOS)
	assert.ErrorIs(t, realDeviceErr, errHideKeyboardNotSupportedOnIOS)
}
