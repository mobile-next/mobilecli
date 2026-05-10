package commands

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices"
	"github.com/mobile-next/mobilecli/utils"
)

// ScreenshotRequest represents the parameters for taking a screenshot
type ScreenshotRequest struct {
	DeviceID   string `json:"deviceId"`
	Format     string `json:"format,omitempty"`     // "png" or "jpeg"
	Quality    int    `json:"quality,omitempty"`    // 1-100, only used for JPEG
	OutputPath string `json:"outputPath,omitempty"` // file path, "-" for stdout, or empty for default naming
}

// ScreenshotResponse represents the response for a screenshot command
type ScreenshotResponse struct {
	Format   string `json:"format"`
	Data     string `json:"data,omitempty"`     // base64 encoded image data
	FilePath string `json:"filePath,omitempty"` // path where file was saved
}

func normalizeFormat(req *ScreenshotRequest) error {
	if req.Format == "" {
		req.Format = "png"
	}
	req.Format = strings.ToLower(req.Format)
	if req.Format != "png" && req.Format != "jpeg" {
		return fmt.Errorf("invalid format '%s'. Supported formats are 'png' and 'jpeg'", req.Format)
	}
	if req.Format == "jpeg" && (req.Quality < 1 || req.Quality > 100) {
		req.Quality = 90
	}
	return nil
}

func resolveFilePath(req ScreenshotRequest, deviceID string) (string, error) {
	if req.OutputPath != "" {
		return filepath.Abs(req.OutputPath)
	}
	ext := "png"
	if req.Format == "jpeg" {
		ext = "jpg"
	}
	safeID := strings.ReplaceAll(deviceID, ":", "_")
	name := fmt.Sprintf("screenshot-%s-%s.%s", safeID, time.Now().Format("20060102150405"), ext)
	return filepath.Abs("./" + name)
}

// ScreenshotCommand takes a screenshot of the specified device
func ScreenshotCommand(req ScreenshotRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	if err := normalizeFormat(&req); err != nil {
		return NewErrorResponse(err)
	}

	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	imageBytes, err := targetDevice.TakeScreenshot()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error taking screenshot: %v", err))
	}

	if req.Format == "jpeg" {
		imageBytes, err = utils.ConvertPngToJpeg(imageBytes, req.Quality)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("error converting to JPEG: %v", err))
		}
	}

	response := ScreenshotResponse{Format: req.Format}

	if req.OutputPath == "-" {
		response.Data = base64.StdEncoding.EncodeToString(imageBytes)
	} else {
		finalPath, err := resolveFilePath(req, targetDevice.ID())
		if err != nil {
			return NewErrorResponse(fmt.Errorf("invalid output path: %v", err))
		}
		if err := os.WriteFile(finalPath, imageBytes, 0o600); err != nil {
			return NewErrorResponse(fmt.Errorf("error writing file: %v", err))
		}
		response.FilePath = finalPath
	}

	return NewSuccessResponse(response)
}
