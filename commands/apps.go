package commands

import (
	"fmt"

	"github.com/mobile-next/mobilecli/devices"
)

// AppRequest represents the parameters for app-related commands
type AppRequest struct {
	DeviceID string `json:"deviceId"`
	BundleID string `json:"bundleId"`
}

// LaunchAppCommand launches an app on the specified device
func LaunchAppCommand(req AppRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.LaunchApp(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to launch app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Launched app '%s' on device %s", req.BundleID, targetDevice.ID()),
	})
}

// TerminateAppCommand terminates an app on the specified device
func TerminateAppCommand(req AppRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.TerminateApp(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to terminate app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Terminated app '%s' on device %s", req.BundleID, targetDevice.ID()),
	})
}

// ListAppsRequest represents the parameters for listing apps
type ListAppsRequest struct {
	DeviceID string `json:"deviceId"`
}

// ListAppsCommand lists installed apps on a device
func ListAppsCommand(req ListAppsRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	apps, err := targetDevice.ListApps()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to list apps on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(apps)
}

// ForegroundAppRequest represents the parameters for getting the foreground app
type ForegroundAppRequest struct {
	DeviceID string `json:"deviceId"`
}

// ForegroundAppCommand gets the currently foreground app on a device
func ForegroundAppCommand(req ForegroundAppRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	// start DeviceKit agent if needed
	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	app, err := targetDevice.GetForegroundApp()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to get foreground app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(app)
}

type InstallAppRequest struct {
	DeviceID string `json:"deviceId"`
	Path     string `json:"path"`
}

func InstallAppCommand(req InstallAppRequest) *CommandResponse {
	if req.Path == "" {
		return NewErrorResponse(fmt.Errorf("path is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = targetDevice.InstallApp(req.Path)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to install app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]interface{}{
		"message": fmt.Sprintf("Installed app from '%s' on device %s", req.Path, targetDevice.ID()),
	})
}

type UninstallAppRequest struct {
	DeviceID    string `json:"deviceId"`
	PackageName string `json:"packageName"`
}

func UninstallAppCommand(req UninstallAppRequest) *CommandResponse {
	if req.PackageName == "" {
		return NewErrorResponse(fmt.Errorf("package name is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	appInfo, err := targetDevice.UninstallApp(req.PackageName)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to uninstall app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(appInfo)
}
