package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// DeviceKitStartRequest represents the parameters for starting DeviceKit
type DeviceKitStartRequest struct {
	DeviceID string `json:"deviceId"`
}

// DeviceKitStartResponse contains information about the started DeviceKit session
type DeviceKitStartResponse struct {
	HTTPPort   int    `json:"httpPort"`
	StreamPort int    `json:"streamPort"`
	Message    string `json:"message"`
}

// DeviceKitStartCommand starts the devicekit-ios XCUITest which provides tap/dumpUI server and broadcast extension
func DeviceKitStartCommand(req DeviceKitStartRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	// Check if this is an iOS device
	iosDevice, ok := targetDevice.(*devices.IOSDevice)
	if !ok {
		return NewErrorResponse(fmt.Errorf("devicekit is only supported on iOS real devices, got %s %s", targetDevice.Platform(), targetDevice.DeviceType()))
	}

	// Start DeviceKit
	info, err := iosDevice.StartDeviceKit()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start devicekit: %v", err))
	}

	return NewSuccessResponse(DeviceKitStartResponse{
		HTTPPort:   info.HTTPPort,
		StreamPort: info.StreamPort,
		Message:    fmt.Sprintf("DeviceKit started on device %s. HTTP server on localhost:%d, H.264 stream on localhost:%d", targetDevice.ID(), info.HTTPPort, info.StreamPort),
	})
}
