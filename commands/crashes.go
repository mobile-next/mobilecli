package commands

import "fmt"

func CrashesListCommand(deviceID string) *CommandResponse {
	crashes, _, err := findCrashReporter(deviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	reports, err := crashes.ListCrashReports()
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error listing crash reports: %w", err))
	}

	return NewSuccessResponse(reports)
}

func CrashesGetCommand(deviceID string, id string) *CommandResponse {
	crashes, _, err := findCrashReporter(deviceID)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error finding device: %w", err))
	}

	content, err := crashes.GetCrashReport(id)
	if err != nil {
		return NewErrorResponse(fmt.Errorf("error getting crash report: %w", err))
	}

	return NewSuccessResponse(map[string]string{
		"id":      id,
		"content": string(content),
	})
}
