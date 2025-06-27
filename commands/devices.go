package commands

import (
	"github.com/mobile-next/mobilecli/devices"
)

// DevicesCommand lists all connected devices
func DevicesCommand() *CommandResponse {
	deviceInfoList, err := devices.GetDeviceInfoList()
	if err != nil {
		return NewErrorResponse(err)
	}

	return NewSuccessResponse(map[string]interface{}{
		"devices": deviceInfoList,
	})
}