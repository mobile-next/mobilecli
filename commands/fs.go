package commands

import "fmt"

type FsPushRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	LocalPath  string `json:"localPath"`
	RemotePath string `json:"remotePath"`
}

func FsPushCommand(req FsPushRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}
	if req.LocalPath == "" {
		return NewErrorResponse(fmt.Errorf("local path is required"))
	}
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	if err := device.PushFile(req.BundleID, req.LocalPath, req.RemotePath); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to push file: %v", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Pushed '%s' to '%s' on app '%s'", req.LocalPath, req.RemotePath, req.BundleID),
	})
}

type FsPullRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
	LocalPath  string `json:"localPath"`
}

func FsPullCommand(req FsPullRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}
	if req.LocalPath == "" {
		return NewErrorResponse(fmt.Errorf("local path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	if err := device.PullFile(req.BundleID, req.RemotePath, req.LocalPath); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to pull file: %v", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Pulled '%s' to '%s' from app '%s'", req.RemotePath, req.LocalPath, req.BundleID),
	})
}

type FsListRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
}

func FsListCommand(req FsListRequest) *CommandResponse {
	if req.BundleID == "" && req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID or remote path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	entries, err := device.ListFiles(req.BundleID, req.RemotePath)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to list files: %v", err))
	}

	return NewSuccessResponse(entries)
}

type FsMkdirRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
}

func FsMkdirCommand(req FsMkdirRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	if err := device.Mkdir(req.BundleID, req.RemotePath); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to create directory: %v", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Created directory '%s' in app '%s'", req.RemotePath, req.BundleID),
	})
}

type FsRmRequest struct {
	DeviceID   string `json:"deviceId"`
	BundleID   string `json:"bundleId"`
	RemotePath string `json:"remotePath"`
}

func FsRmCommand(req FsRmRequest) *CommandResponse {
	if req.BundleID == "" {
		return NewErrorResponse(fmt.Errorf("bundle ID is required"))
	}
	if req.RemotePath == "" {
		return NewErrorResponse(fmt.Errorf("remote path is required"))
	}

	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	if err := device.Rm(req.BundleID, req.RemotePath); err != nil {
		return NewErrorResponse(fmt.Errorf("failed to remove: %v", err))
	}

	return NewSuccessResponse(map[string]any{
		"message": fmt.Sprintf("Removed '%s' from app '%s'", req.RemotePath, req.BundleID),
	})
}
