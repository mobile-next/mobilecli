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

// ScreenshotCommand takes a screenshot of the specified device
func ScreenshotCommand(req ScreenshotRequest) *CommandResponse {
	// Find the target device
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %v", err))
	}

	// Set default format
	if req.Format == "" {
		req.Format = "png"
	}

	// Validate format
	req.Format = strings.ToLower(req.Format)
	if req.Format != "png" && req.Format != "jpeg" {
		return NewErrorResponse(fmt.Errorf("invalid format '%s'. Supported formats are 'png' and 'jpeg'", req.Format))
	}

	// Validate JPEG quality
	if req.Format == "jpeg" {
		if req.Quality < 1 || req.Quality > 100 {
			req.Quality = 90 // Default quality
		}
	}

	// Start agent if needed
	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	// Take screenshot
	imageBytes, err := targetDevice.TakeScreenshot()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error taking screenshot: %v", err))
	}

	// Convert to JPEG if requested
	if req.Format == "jpeg" {
		convertedBytes, err := utils.ConvertPngToJpeg(imageBytes, req.Quality)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("error converting to JPEG: %v", err))
		}
		imageBytes = convertedBytes
	}

	response := ScreenshotResponse{
		Format: req.Format,
	}

	// Handle output
	if req.OutputPath == "-" {
		// Return as base64 data for stdout
		response.Data = base64.StdEncoding.EncodeToString(imageBytes)
	} else {
		// Save to file
		var finalPath string
		if req.OutputPath != "" {
			finalPath, err = filepath.Abs(req.OutputPath)
			if err != nil {
				return NewErrorResponse(fmt.Errorf("invalid output path: %v", err))
			}
		} else {
			// Default filename generation
			timestamp := time.Now().Format("20060102150405")
			safeDeviceID := strings.ReplaceAll(targetDevice.ID(), ":", "_")
			extension := "png"
			if req.Format == "jpeg" {
				extension = "jpg"
			}
			fileName := fmt.Sprintf("screenshot-%s-%s.%s", safeDeviceID, timestamp, extension)
			finalPath, err = filepath.Abs("./" + fileName)
			if err != nil {
				return NewErrorResponse(fmt.Errorf("error creating default path: %v", err))
			}
		}

		// Write file
		err = os.WriteFile(finalPath, imageBytes, 0o600)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("error writing file: %v", err))
		}

		response.FilePath = finalPath
	}

	return NewSuccessResponse(response)
}
