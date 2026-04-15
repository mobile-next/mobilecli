package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mobile-next/mobilecli/devices"
)

type LogsRequest struct {
	DeviceID string
	Limit    int
	Process  string
	PID      int
}

func LogsCommand(req LogsRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	encoder := json.NewEncoder(os.Stdout)
	count := 0
	err = device.StreamLogs(func(entry devices.LogEntry) bool {
		if req.Process != "" && !strings.Contains(entry.Process, req.Process) {
			return true
		}
		if req.PID >= 0 && entry.PID != req.PID {
			return true
		}

		if err := encoder.Encode(entry); err != nil {
			return false
		}
		count++
		if req.Limit > 0 && count >= req.Limit {
			return false
		}
		return true
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error streaming logs: %w", err))
	}

	return NewSuccessResponse("done")
}
