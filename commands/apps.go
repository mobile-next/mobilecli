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
	Activity string   `json:"activity,omitempty"`
}

// LaunchAppCommand launches an app on the specified device
func LaunchAppCommand(req AppRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	apps, targetDevice, err := findAppManager(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = apps.LaunchApp(req.BundleID, devices.LaunchOptions{Locales: req.Locales, Activity: req.Activity})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to launch app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Launched app '%s' on device %s", req.BundleID, targetDevice.ID()),
	})
}

// TerminateAppCommand terminates an app on the specified device
func TerminateAppCommand(req AppRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	apps, targetDevice, err := findAppManager(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	err = apps.TerminateApp(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to terminate app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(MessageResult{
		Message: fmt.Sprintf("Terminated app '%s' on device %s", req.BundleID, targetDevice.ID()),
	})
}

// ListAppsRequest represents the parameters for listing apps
type ListAppsRequest struct {
	DeviceID string `json:"deviceId"`
}

// ListAppsCommand lists installed apps on a device
func ListAppsCommand(req ListAppsRequest) *CommandResponse {
	apps, targetDevice, err := findAppManager(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	installed, err := apps.ListApps(true)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to list apps on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(installed)
}

// ForegroundAppRequest represents the parameters for getting the foreground app
type ForegroundAppRequest struct {
	DeviceID string `json:"deviceId"`
}

// ForegroundAppCommand gets the currently foreground app on a device
func ForegroundAppCommand(req ForegroundAppRequest) *CommandResponse {
	targetDevice, err := FindDeviceWithAgent(req.DeviceID)
	if err != nil {
		return NewErrorResponse(err)
	}

	apps, err := requireCapability[devices.AppManager](targetDevice, "app management")
	if err != nil {
		return NewErrorResponse(err)
	}

	app, err := apps.GetForegroundApp()
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

// InstallAppResult is returned on a successful install, including the app
// metadata parsed from the installed file when available.
type InstallAppResult struct {
	Message string             `json:"message"`
	App     *utils.AppMetadata `json:"app,omitempty"`
}

func InstallAppCommand(req InstallAppRequest) *CommandResponse {
	if req.Path == "" {
		return NewErrorResponse(fmt.Errorf("path is required"))
	}

	apps, targetDevice, err := findAppManager(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	installPath := req.Path

	// re-sign IPA if requested, only for .ipa files on real iOS devices
	if req.ForceResign {
		if !strings.HasSuffix(strings.ToLower(req.Path), ".ipa") {
			return NewErrorResponse(fmt.Errorf("--force-resign only works with .ipa files"))
		}

		if targetDevice.Platform() != devices.PlatformIOS || targetDevice.DeviceType() != devices.DeviceTypeReal {
			return NewErrorResponse(fmt.Errorf("--force-resign only works with real iOS devices"))
		}

		resignedPath, err := utils.ResignIPA(req.Path, targetDevice.ID(), req.ProvisioningProfile, req.SigningIdentity)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("failed to re-sign IPA: %w", err))
		}
		defer func() { _ = os.Remove(resignedPath) }()

		installPath = resignedPath
	}

	err = apps.InstallApp(installPath)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to install app on device %s: %w", targetDevice.ID(), err))
	}

	result := InstallAppResult{
		Message: fmt.Sprintf("Installed app from '%s' on device %s", req.Path, targetDevice.ID()),
	}

	// metadata extraction is best-effort: a parse failure must not turn a
	// successful install into an error.
	if meta, err := utils.ParseAppMetadata(req.Path); err != nil {
		utils.Verbose("failed to parse app metadata from %s: %v", req.Path, err)
	} else {
		result.App = meta
	}

	return NewSuccessResponse(result)
}

type AppPathRequest struct {
	DeviceID string `json:"deviceId"`
	BundleID string `json:"bundleId"`
}

func AppPathCommand(req AppPathRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	apps, device, err := findAppManager(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	path, err := apps.GetAppContainerPath(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to get app path on device %s: %w", device.ID(), err))
	}

	return NewSuccessResponse(map[string]any{
		"path": path,
	})
}

type ClearAppRequest struct {
	DeviceID string `json:"deviceId"`
	BundleID string `json:"bundleId"`
}

func ClearAppCommand(req ClearAppRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}

	apps, targetDevice, err := findAppManager(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	err = apps.ClearApp(req.BundleID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to clear app on device %s: %w", targetDevice.ID(), err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Cleared app '%s' on device %s", req.BundleID, targetDevice.ID()),
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

	apps, targetDevice, err := findAppManager(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	appInfo, err := apps.UninstallApp(req.PackageName)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to uninstall app on device %s: %v", targetDevice.ID(), err))
	}

	return NewSuccessResponse(appInfo)
}
