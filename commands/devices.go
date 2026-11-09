package commands

import (
	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

// shouldEvictAfterListing reports whether a listing is complete enough to
// decide which cached devices are gone: unfiltered, and with the remote source
// answering (a failed fetch must not evict remote devices). Offline devices
// are re-found on the next lookup miss anyway.
func shouldEvictAfterListing(opts devices.DeviceListOptions, remoteFailed bool) bool {
	return opts.Platform == "" && opts.DeviceType == "" && !remoteFailed
}

// DevicesCommand lists all connected devices, merging remote devices if a token is provided
func DevicesCommand(opts devices.DeviceListOptions, token string) *CommandResponse {
	deviceInfoList, err := devices.GetDeviceInfoList(opts)
	if err != nil {
		return NewErrorResponse(err)
	}

	remoteFailed := false
	if token != "" {
		remoteDevices, err := FetchRemoteDevices(token)
		if err != nil {
			utils.Verbose("failed to fetch remote devices: %v", err)
			remoteFailed = true
		} else {
			deviceInfoList = append(deviceInfoList, remoteDevices...)
		}
	}

	if shouldEvictAfterListing(opts, remoteFailed) {
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
