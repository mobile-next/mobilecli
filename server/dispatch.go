package server

import (
	"encoding/json"
	"fmt"

	"github.com/mobile-next/mobilecli/commands"
)

// HandlerFunc is the signature for non-streaming JSON-RPC method handlers
type HandlerFunc func(params json.RawMessage) (interface{}, error)

// GetMethodRegistry returns a map of method names to handler functions
// This is used by both the HTTP server and embedded clients
func GetMethodRegistry() map[string]HandlerFunc {
	return map[string]HandlerFunc{
		"devices":            handleDevicesList,
		"screenshot":         handleScreenshot,
		"io_tap":             handleIoTap,
		"io_longpress":       handleIoLongPress,
		"io_text":            handleIoText,
		"io_button":          handleIoButton,
		"io_swipe":           handleIoSwipe,
		"io_gesture":         handleIoGesture,
		"url":                handleURL,
		"device_info":        handleDeviceInfo,
		"io_orientation_get": handleIoOrientationGet,
		"io_orientation_set": handleIoOrientationSet,
		"device_shutdown":    handleDeviceShutdown,
		"device_reboot":      handleDeviceReboot,
		"dump_ui":            handleDumpUI,
		"apps_launch":        handleAppsLaunch,
		"apps_terminate":     handleAppsTerminate,
		"apps_list":          handleAppsList,
		"device_boot":        wrapDeviceBootHandler(),
	}
}

// wrapDeviceBootHandler creates a wrapper that matches HandlerFunc signature
func wrapDeviceBootHandler() HandlerFunc {
	return func(params json.RawMessage) (interface{}, error) {
		return handleDeviceBootWithoutTimeout(params)
	}
}

// handleDeviceBootWithoutTimeout is device_boot handler without HTTP-specific timeout logic
func handleDeviceBootWithoutTimeout(params json.RawMessage) (interface{}, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("'params' is required with fields: deviceId")
	}

	var bootParams DeviceBootParams
	err := json.Unmarshal(params, &bootParams)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %v. Expected fields: deviceId", err)
	}

	req := commands.BootRequest{
		DeviceID: bootParams.DeviceID,
	}

	response := commands.BootCommand(req)
	if response.Status == "error" {
		return nil, fmt.Errorf("%s", response.Error)
	}

	return response.Data, nil
}

// Execute dispatches a method call using the registry
// This is the main entry point for embedded clients
func Execute(method string, params json.RawMessage) (interface{}, error) {
	registry := GetMethodRegistry()

	handler, exists := registry[method]
	if !exists {
		return nil, fmt.Errorf("method not found: %s", method)
	}

	return handler(params)
}
