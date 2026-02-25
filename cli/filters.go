package cli

import (
	"fmt"
	"strings"

	"github.com/mobile-next/mobilecli/commands"
)

// parseVersionFilter parses a version flag value into a DeviceFilter.
// Supports: ">=18", ">18", "<=20", "<20", "18.6.2" (exact match).
func parseVersionFilter(value string) (commands.DeviceFilter, error) {
	value = strings.TrimSpace(value)
	f := commands.DeviceFilter{Attribute: "version"}

	switch {
	case strings.HasPrefix(value, ">="):
		f.Operator = "GREATER_THAN_OR_EQUALS"
		f.Value = strings.TrimSpace(value[2:])
	case strings.HasPrefix(value, ">"):
		f.Operator = "GREATER_THAN"
		f.Value = strings.TrimSpace(value[1:])
	case strings.HasPrefix(value, "<="):
		f.Operator = "LESS_THAN_OR_EQUALS"
		f.Value = strings.TrimSpace(value[2:])
	case strings.HasPrefix(value, "<"):
		f.Operator = "LESS_THAN"
		f.Value = strings.TrimSpace(value[1:])
	default:
		f.Operator = "EQUALS"
		f.Value = value
	}

	if f.Value == "" {
		return f, fmt.Errorf("version filter value cannot be empty")
	}

	return f, nil
}

// parseNameFilter parses a name flag value into a DeviceFilter.
// Supports: "iPhone*" (starts with), "iPhone 16" (exact match).
func parseNameFilter(value string) (commands.DeviceFilter, error) {
	value = strings.TrimSpace(value)
	f := commands.DeviceFilter{Attribute: "name"}

	if strings.HasSuffix(value, "*") {
		f.Operator = "STARTS_WITH"
		f.Value = strings.TrimSuffix(value, "*")
	} else {
		f.Operator = "EQUALS"
		f.Value = value
	}

	if f.Value == "" {
		return f, fmt.Errorf("name filter value cannot be empty")
	}

	return f, nil
}

// buildAllocateFilters builds a filters slice from CLI flag values.
func buildAllocateFilters(platform, deviceType string, versions, names []string) ([]commands.DeviceFilter, error) {
	var filters []commands.DeviceFilter

	filters = append(filters, commands.DeviceFilter{
		Attribute: "platform",
		Operator:  "EQUALS",
		Value:     platform,
	})

	if deviceType != "" {
		filters = append(filters, commands.DeviceFilter{
			Attribute: "type",
			Operator:  "EQUALS",
			Value:     deviceType,
		})
	}

	for _, v := range versions {
		f, err := parseVersionFilter(v)
		if err != nil {
			return nil, fmt.Errorf("invalid --version %q: %w", v, err)
		}
		filters = append(filters, f)
	}

	for _, n := range names {
		f, err := parseNameFilter(n)
		if err != nil {
			return nil, fmt.Errorf("invalid --name %q: %w", n, err)
		}
		filters = append(filters, f)
	}

	return filters, nil
}
