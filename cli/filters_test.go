package cli

import (
	"testing"

	"github.com/mobile-next/mobilecli/commands"
)

func TestParseVersionFilter_GreaterThanOrEquals(t *testing.T) {
	f, err := parseVersionFilter(">=18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Operator != "GREATER_THAN_OR_EQUALS" || f.Value != "18" {
		t.Errorf("expected GTE 18, got %s %s", f.Operator, f.Value)
	}
}

func TestParseVersionFilter_GreaterThan(t *testing.T) {
	f, err := parseVersionFilter(">18")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Operator != "GREATER_THAN" || f.Value != "18" {
		t.Errorf("expected GT 18, got %s %s", f.Operator, f.Value)
	}
}

func TestParseVersionFilter_LessThanOrEquals(t *testing.T) {
	f, err := parseVersionFilter("<=20")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Operator != "LESS_THAN_OR_EQUALS" || f.Value != "20" {
		t.Errorf("expected LTE 20, got %s %s", f.Operator, f.Value)
	}
}

func TestParseVersionFilter_LessThan(t *testing.T) {
	f, err := parseVersionFilter("<20")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Operator != "LESS_THAN" || f.Value != "20" {
		t.Errorf("expected LT 20, got %s %s", f.Operator, f.Value)
	}
}

func TestParseVersionFilter_ExactMatch(t *testing.T) {
	f, err := parseVersionFilter("18.6.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Operator != "EQUALS" || f.Value != "18.6.2" {
		t.Errorf("expected EQUALS 18.6.2, got %s %s", f.Operator, f.Value)
	}
}

func TestParseVersionFilter_EmptyAfterPrefix(t *testing.T) {
	_, err := parseVersionFilter(">=")
	if err == nil {
		t.Fatal("expected error for empty value after prefix")
	}
}

func TestParseNameFilter_StartsWith(t *testing.T) {
	f, err := parseNameFilter("iPhone*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Operator != "STARTS_WITH" || f.Value != "iPhone" {
		t.Errorf("expected STARTS_WITH iPhone, got %s %s", f.Operator, f.Value)
	}
}

func TestParseNameFilter_ExactMatch(t *testing.T) {
	f, err := parseNameFilter("iPhone 16")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Operator != "EQUALS" || f.Value != "iPhone 16" {
		t.Errorf("expected EQUALS 'iPhone 16', got %s %s", f.Operator, f.Value)
	}
}

func TestParseNameFilter_JustWildcard(t *testing.T) {
	_, err := parseNameFilter("*")
	if err == nil {
		t.Fatal("expected error for wildcard-only name")
	}
}

func TestBuildAllocateFilters_PlatformOnly(t *testing.T) {
	filters, err := buildAllocateFilters("ios", "", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}
	if filters[0].Attribute != "platform" || filters[0].Value != "ios" {
		t.Errorf("expected platform=ios, got %s=%s", filters[0].Attribute, filters[0].Value)
	}
}

func TestBuildAllocateFilters_WithType(t *testing.T) {
	filters, err := buildAllocateFilters("ios", "real", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}
	if filters[1].Attribute != "type" || filters[1].Value != "real" {
		t.Errorf("expected type=real, got %s=%s", filters[1].Attribute, filters[1].Value)
	}
}

func TestBuildAllocateFilters_Combined(t *testing.T) {
	filters, err := buildAllocateFilters("ios", "real", []string{">=18", "<20"}, []string{"iPhone*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 5 {
		t.Fatalf("expected 5 filters, got %d", len(filters))
	}

	expectFilter(t, filters[0], "platform", "EQUALS", "ios")
	expectFilter(t, filters[1], "type", "EQUALS", "real")
	expectFilter(t, filters[2], "version", "GREATER_THAN_OR_EQUALS", "18")
	expectFilter(t, filters[3], "version", "LESS_THAN", "20")
	expectFilter(t, filters[4], "name", "STARTS_WITH", "iPhone")
}

func expectFilter(t *testing.T, f commands.DeviceFilter, attr, op, val string) {
	t.Helper()
	if f.Attribute != attr || f.Operator != op || f.Value != val {
		t.Errorf("expected {%s %s %s}, got {%s %s %s}", attr, op, val, f.Attribute, f.Operator, f.Value)
	}
}
