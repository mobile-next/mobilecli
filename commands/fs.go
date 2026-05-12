package commands

import "fmt"

type FsPushRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	LocalPath  string `json:"localPath"`
	RemotePath string `json:"remotePath"`
}

func FsPushCommand(req FsPushRequest) *CommandResponse {
	if req.LocalPath == "" {
		return NewErrorResponse(fmt.Errorf("local path is required"))
	}
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	if err := device.PushFile(req.BundleID, req.LocalPath, req.RemotePath); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to push file: %w", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Pushed '%s' to '%s'", req.LocalPath, req.RemotePath),
	})
}

type FsPullRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
	LocalPath  string `json:"localPath"`
}

func FsPullCommand(req FsPullRequest) *CommandResponse {
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}
	if req.LocalPath == "" {
		return NewErrorResponse(fmt.Errorf("local path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	if err := device.PullFile(req.BundleID, req.RemotePath, req.LocalPath); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to pull file: %w", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Pulled '%s' to '%s'", req.RemotePath, req.LocalPath),
	})
}

type FsListRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
}

func FsListCommand(req FsListRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	entries, err := device.ListFiles(req.BundleID, req.RemotePath)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to list files: %w", err))
	}

	return NewSuccessResponse(entries)
}

type FsMkdirRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
	Parents    bool   `json:"parents"`
}

func FsMkdirCommand(req FsMkdirRequest) *CommandResponse {
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	if err := device.Mkdir(req.BundleID, req.RemotePath, req.Parents); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to create directory: %w", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Created directory '%s'", req.RemotePath),
	})
}

type FsRmRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
	Recursive  bool   `json:"recursive"`
}

func FsRmCommand(req FsRmRequest) *CommandResponse {
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	if err := device.Rm(req.BundleID, req.RemotePath, req.Recursive); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to remove: %w", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Removed '%s'", req.RemotePath),
	})
}
