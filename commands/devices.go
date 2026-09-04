package commands

import (
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

// DevicesCommand lists all connected devices, merging remote devices if a token is provided
func DevicesCommand(opts devices.DeviceListOptions, token string) *CommandResponse {
	deviceInfoList, err := devices.GetDeviceInfoList(opts)
	if err != nil {
		return NewErrorResponse(err)
	}

	if token != "" {
		remoteDevices, err := FetchRemoteDevices(token)
		if err != nil {
			utils.Verbose("failed to fetch remote devices: %v", err)
		} else {
			deviceInfoList = append(deviceInfoList, remoteDevices...)
		}
	}

	// an unfiltered listing is the one moment we know which cached devices are
	// gone (offline devices are re-found on the next lookup miss anyway)
	if opts.Platform == "" && opts.DeviceType == "" {
		ids := make([]string, 0, len(deviceInfoList))
		for _, d := range deviceInfoList {
			ids = append(ids, d.ID)
		}
		EvictDevicesNotIn(ids)
	}

	return NewSuccessResponse(map[string]any{
		"devices": deviceInfoList,
	})
}
