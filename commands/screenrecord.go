package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mobile-next/mobilecli/devices"
)

// ScreenRecordRequest represents the parameters for recording video
type ScreenRecordRequest struct {
	DeviceID   string `json:"deviceId"`
	BitRate    string `json:"bitRate,omitempty"`
	TimeLimit  int    `json:"timeLimit,omitempty"`
	OutputPath string `json:"outputPath,omitempty"`
}

// ScreenRecordResponse represents the response for a screenrecord command
type ScreenRecordResponse struct {
	FilePath string `json:"filePath"`
}

// parseBitRate parses a bit-rate string like "4M", "500K", or "8000000" into an integer
func parseBitRate(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty bit-rate string")
	}

	upper := strings.ToUpper(s)

	if strings.HasSuffix(upper, "M") {
		num, err := strconv.ParseFloat(upper[:len(upper)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid bit-rate: %w", err)
		}
		return int(num * 1_000_000), nil
	}

	if strings.HasSuffix(upper, "K") {
		num, err := strconv.ParseFloat(upper[:len(upper)-1], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid bit-rate: %w", err)
		}
		return int(num * 1_000), nil
	}

	num, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid bit-rate: %w", err)
	}
	return num, nil
}

// ScreenRecordCommand records video from the specified device
func ScreenRecordCommand(req ScreenRecordRequest) *CommandResponse {
	targetDevice, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	// parse bit-rate (default 8M)
	bitRateStr := req.BitRate
	if bitRateStr == "" {
		bitRateStr = "8M"
	}

	bitRate, err := parseBitRate(bitRateStr)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("invalid bit-rate '%s': %w", req.BitRate, err))
	}

	// validate time-limit (default 300, max 300)
	timeLimit := req.TimeLimit
	if timeLimit <= 0 {
		timeLimit = 300
	}
	if timeLimit > 300 {
		return NewErrorResponse(fmt.Errorf("time-limit must be at most 300 seconds, got %d", timeLimit))
	}

	// determine output path
	outputPath := req.OutputPath
	if outputPath == "" {
		timestamp := time.Now().Format("20060102150405")
		safeDeviceID := strings.ReplaceAll(targetDevice.ID(), ":", "_")
		fileName := fmt.Sprintf("screenrecord-%s-%s.mp4", safeDeviceID, timestamp)
		outputPath, err = filepath.Abs("./" + fileName)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("error creating default path: %w", err))
		}
	} else {
		outputPath, err = filepath.Abs(outputPath)
		if err != nil {
			return NewErrorResponse(fmt.Errorf("invalid output path: %w", err))
		}
	}

	// start agent
	err = targetDevice.StartAgent(devices.StartAgentConfig{
		Hook: GetShutdownHook(),
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("failed to start agent on device %s: %w", targetDevice.ID(), err))
	}

	// record video
	err = targetDevice.RecordVideo(devices.RecordVideoConfig{
		BitRate:    bitRate,
		TimeLimit:  timeLimit,
		OutputPath: outputPath,
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error recording video: %w", err))
	}

	return NewSuccessResponse(ScreenRecordResponse{
		FilePath: outputPath,
	})
}
