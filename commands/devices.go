package commands

import (
	"github.com/mobile-next/mobilecli/devices"
)

// DevicesCommand lists all connected devices
func DevicesCommand(opts devices.DeviceListOptions) *CommandResponse {
	deviceInfoList, err := devices.GetDeviceInfoList(opts)
	if err != nil {
		return NewErrorResponse(err)
	}

	return NewSuccessResponse(map[string]interface{}{
		"devices": deviceInfoList,
	})
}
