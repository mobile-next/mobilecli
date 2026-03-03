package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

// AppRequest represents the parameters for app-related commands
type AppRequest struct {
	DeviceID string   `json:"deviceId"`
	BundleID string   `json:"bundleId"`
	Locales  []string `json:"locales,omitempty"`
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

	err = targetDevice.LaunchApp(req.BundleID, req.Locales)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to launch app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]any{
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

	return NewSuccessResponse(map[string]any{
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

	// start agent if needed (for WDA)
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
	DeviceID            string `json:"deviceId"`
	Path                string `json:"path"`
	ForceResign         bool   `json:"forceResign"`
	ProvisioningProfile string `json:"provisioningProfile"`
	SigningIdentity     string `json:"signingIdentity"`
}

func InstallAppCommand(req InstallAppRequest) *CommandResponse {
	if req.Path == "" {
		return NewErrorResponse(fmt.Errorf("path is required"))
	}

	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	installPath := req.Path

	// re-sign IPA if requested, only for .ipa files on real iOS devices
	if req.ForceResign {
		if !strings.HasSuffix(strings.ToLower(req.Path), ".ipa") {
			return NewErrorResponse(fmt.Errorf("--force-resign only works with .ipa files"))
		}

		if targetDevice.Platform() != "ios" || targetDevice.DeviceType() != "real" {
			return NewErrorResponse(fmt.Errorf("--force-resign only works with real iOS devices"))
		}

		resignedPath, err := utils.ResignIPA(req.Path, targetDevice.ID(), req.ProvisioningProfile, req.SigningIdentity)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("failed to re-sign IPA: %w", err))
		}
		defer func() { _ = os.Remove(resignedPath) }()

		installPath = resignedPath
	}

	err = targetDevice.InstallApp(installPath)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to install app on device %s: %w", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]any{
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
