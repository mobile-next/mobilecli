package commands

import (
	"github.com/mobile-next/mobilecli/devices"
)

// DevicesCommand lists all connected devices
func DevicesCommand(showAll bool, platform string, deviceType string) *CommandResponse {
	deviceInfoList, err := devices.GetDeviceInfoList(showAll, platform, deviceType)
	if err != nil {
		return NewErrorResponse(err)
	}

	return NewSuccessResponse(map[string]interface{}{
		"devices": deviceInfoList,
	})
}
