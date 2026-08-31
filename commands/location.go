package commands

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/devices"
)

// LocationSetRequest represents the request for overriding the device location
type LocationSetRequest struct {
	DeviceID  string  `json:"deviceId"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// LocationClearRequest represents the request for clearing a location override
type LocationClearRequest struct {
	DeviceID string `json:"deviceId"`
}

// LocationResponse represents the response containing the simulated location
type LocationResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ParseLatLon parses a "latitude,longitude" pair, as accepted by the cli and
// by `xcrun simctl location set`.
func ParseLatLon(s string) (float64, float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid coordinates '%s', expected format 'latitude,longitude'", s)
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude '%s'", strings.TrimSpace(parts[0]))
	}

	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude '%s'", strings.TrimSpace(parts[1]))
	}

	if err := validateCoordinates(lat, lon); err != nil {
		return 0, 0, err
	}

	return lat, lon, nil
}

// validateCoordinates rejects coordinates no device can be placed at. Every
// caller goes through here, the cli via ParseLatLon and json-rpc directly.
func validateCoordinates(lat, lon float64) error {
	if math.IsNaN(lat) || math.IsInf(lat, 0) {
		return fmt.Errorf("latitude %v is not a valid number", lat)
	}

	if math.IsNaN(lon) || math.IsInf(lon, 0) {
		return fmt.Errorf("longitude %v is not a valid number", lon)
	}

	if lat < -90 || lat > 90 {
		return fmt.Errorf("latitude %v out of range, must be between -90 and 90", lat)
	}

	if lon < -180 || lon > 180 {
		return fmt.Errorf("longitude %v out of range, must be between -180 and 180", lon)
	}

	return nil
}

// LocationSetCommand overrides the device location
func LocationSetCommand(req LocationSetRequest) *CommandResponse {
	if err := validateCoordinates(req.Latitude, req.Longitude); err != nil {
		return NewErrorResponse(err)
	}

	settable, err := findLocationSettableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if err := settable.SetLocation(req.Latitude, req.Longitude); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to set location: %v", err))
	}

	return NewSuccessResponse(LocationResponse{
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	})
}

// LocationClearCommand removes a location override, restoring the real location
func LocationClearCommand(req LocationClearRequest) *CommandResponse {
	settable, err := findLocationSettableDevice(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if err := settable.ClearLocation(); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to clear location: %v", err))
	}

	return NewSuccessResponse(OK)
}

// findLocationSettableDevice resolves a device and asserts it can simulate a location
func findLocationSettableDevice(deviceID string) (devices.LocationSettable, error) {
	device, err := FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return nil, err
	}

	settable, ok := device.(devices.LocationSettable)
	if !ok {
		return nil, fmt.Errorf("location override is not supported on %s %s devices", device.Platform(), device.DeviceType())
	}

	return settable, nil
}
