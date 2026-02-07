package server

import (
	"encoding/json"
	"fmt"
)

// HandlerFunc is the signature for non-streaming JSON-RPC method handlers
type HandlerFunc func(params json.RawMessage) (interface{}, error)

// GetMethodRegistry returns a map of method names to handler functions
// This is used by both the HTTP server and embedded clients
func GetMethodRegistry() map[string]HandlerFunc {
	return map[string]HandlerFunc{
		"devices.list":              handleDevicesList,
		"device.screenshot":         handleScreenshot,
		"device.screencapture":      handleScreenCaptureSession,
		"device.io.tap":             handleIoTap,
		"device.io.longpress":       handleIoLongPress,
		"device.io.text":            handleIoText,
		"device.io.button":          handleIoButton,
		"device.io.swipe":           handleIoSwipe,
		"device.io.gesture":         handleIoGesture,
		"device.url":                handleURL,
		"device.info":               handleDeviceInfo,
		"device.io.orientation.get": handleIoOrientationGet,
		"device.io.orientation.set": handleIoOrientationSet,
		"device.boot":               handleDeviceBoot,
		"device.shutdown":           handleDeviceShutdown,
		"device.reboot":             handleDeviceReboot,
		"device.dump.ui":            handleDumpUI,
		"device.apps.launch":        handleAppsLaunch,
		"device.apps.terminate":     handleAppsTerminate,
		"device.apps.list":          handleAppsList,
		"device.apps.foreground":    handleAppsForeground,
	}
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
