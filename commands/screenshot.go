package commands

import (
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices"
)

// ScreenshotRequest represents the parameters for taking a screenshot
type ScreenshotRequest struct {
	DeviceID   string  `json:"deviceId"`
	Format     string  `json:"format,omitempty"`     // "png" or "jpeg"
	Quality    int     `json:"quality,omitempty"`    // 1-100, only used for JPEG
	Scale      float64 `json:"scale,omitempty"`      // 0.0-1.0, 0 or 1.0 means no scaling
	MaxSize    int     `json:"maxSize,omitempty"`    // max(width, height) in pixels, takes precedence over Scale, 0 means no limit
	OutputPath string  `json:"outputPath,omitempty"` // file path, "-" for stdout, or empty for default naming
}

// ScreenshotResponse represents the response for a screenshot command
type ScreenshotResponse struct {
	Format   string `json:"format"`
	Data     string `json:"data,omitempty"`     // base64 encoded image data
	FilePath string `json:"filePath,omitempty"` // path where file was saved
}

// validateScreenshotResize validates scale and maxSize, defaulting a zero
// scale to 1.0 (no scaling).
func validateScreenshotResize(scale float64, maxSize int) (float64, error) {
	if scale == 0 {
		scale = 1.0
	}
	if math.IsNaN(scale) || scale < 0 || scale > 1.0 {
		return 0, fmt.Errorf("scale must be between 0.0 and 1.0")
	}
	if maxSize < 0 {
		return 0, fmt.Errorf("maxSize must be a positive number of pixels")
	}
	return scale, nil
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

	// Validate scale and maxSize
	scale, err := validateScreenshotResize(req.Scale, req.MaxSize)
	if err != nil {
		return NewErrorResponse(err)
	}
	req.Scale = scale

	// Start agent if needed
	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %v", targetDevice.ID(), err))
	}

	// Take screenshot; the device is responsible for format, quality, and scaling
	imageBytes, err := targetDevice.TakeScreenshot(devices.ScreenshotOptions{
		Format:  req.Format,
		Quality: req.Quality,
		Scale:   req.Scale,
		MaxSize: req.MaxSize,
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error taking screenshot: %v", err))
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
