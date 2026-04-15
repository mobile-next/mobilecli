package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mobile-next/mobilecli/devices"
)

type LogsRequest struct {
	DeviceID string
	Limit    int
}

func LogsCommand(req LogsRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	encoder := json.NewEncoder(os.Stdout)
	count := 0
	err = device.StreamLogs(func(entry devices.LogEntry) bool {
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
