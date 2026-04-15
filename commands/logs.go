package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mobile-next/mobilecli/devices"
)

type LogFilter struct {
	Key    string
	Value  string
	Negate bool
}

type LogsRequest struct {
	DeviceID string
	Limit    int
	Filters  []LogFilter
}

// ParseLogFilters parses filter strings like "key=value" or "key!=value"
func ParseLogFilters(raw []string) ([]LogFilter, error) {
	var filters []LogFilter
	for _, s := range raw {
		f, err := parseOneFilter(s)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}

var validFilterKeys = map[string]bool{
	"pid": true, "process": true, "tag": true,
	"level": true, "subsystem": true, "category": true,
	"message": true,
}

func parseOneFilter(s string) (LogFilter, error) {
	// try != first (before =)
	if idx := strings.Index(s, "!="); idx > 0 {
		key := s[:idx]
		if !validFilterKeys[key] {
			return LogFilter{}, fmt.Errorf("unknown filter key %q (valid: pid, process, tag, level, subsystem, category, message)", key)
		}
		return LogFilter{Key: key, Value: s[idx+2:], Negate: true}, nil
	}
	if idx := strings.Index(s, "="); idx > 0 {
		key := s[:idx]
		if !validFilterKeys[key] {
			return LogFilter{}, fmt.Errorf("unknown filter key %q (valid: pid, process, tag, level, subsystem, category, message)", key)
		}
		return LogFilter{Key: key, Value: s[idx+1:]}, nil
	}
	return LogFilter{}, fmt.Errorf("invalid filter %q (expected key=value or key!=value)", s)
}

func getFieldValue(entry devices.LogEntry, key string) string {
	switch key {
	case "pid":
		return strconv.Itoa(entry.PID)
	case "process":
		return entry.Process
	case "tag":
		return entry.Tag
	case "level":
		return entry.Level
	case "subsystem":
		return entry.Subsystem
	case "category":
		return entry.Category
	case "message":
		return entry.Message
	default:
		return ""
	}
}

func matchesFilters(entry devices.LogEntry, filters []LogFilter) bool {
	for _, f := range filters {
		fieldValue := getFieldValue(entry, f.Key)
		match := fieldValue == f.Value
		if f.Negate {
			match = !match
		}
		if !match {
			return false
		}
	}
	return true
}

func LogsCommand(req LogsRequest) *CommandResponse {
	device, err := FindDeviceOrAutoSelect(req.DeviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	encoder := json.NewEncoder(os.Stdout)
	count := 0

	emit := func(entry devices.LogEntry) bool {
		if err := encoder.Encode(entry); err != nil {
			return false
		}
		count++
		if req.Limit > 0 && count >= req.Limit {
			return false
		}
		return true
	}

	err = device.StreamLogs(func(entry devices.LogEntry) bool {
		if !matchesFilters(entry, req.Filters) {
			return true
		}
		return emit(entry)
	})
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error streaming logs: %w", err))
	}

	return NewSuccessResponse("done")
}
