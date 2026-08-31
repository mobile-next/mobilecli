package commands

import (
	"testing"

	"github.com/mobile-next/mobilecli/devices"
)

func TestParseLogFiltersReadsIncludeFilters(t *testing.T) {
	filters, err := ParseLogFilters([]string{"process=SpringBoard", "level=Error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []LogFilter{
		{Key: "process", Value: "SpringBoard"},
		{Key: "level", Value: "Error"},
	}
	assertFiltersEqual(t, filters, expected)
}

func TestParseLogFiltersReadsExcludeFiltersWithoutLeakingBangIntoKey(t *testing.T) {
	filters, err := ParseLogFilters([]string{"process!=SpringBoard"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFiltersEqual(t, filters, []LogFilter{
		{Key: "process", Value: "SpringBoard", Negate: true},
	})
}

func TestParseLogFiltersKeepsEqualsSignsInsideValue(t *testing.T) {
	filters, err := ParseLogFilters([]string{"message=a=b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFiltersEqual(t, filters, []LogFilter{{Key: "message", Value: "a=b"}})
}

func TestParseLogFiltersRejectsBadInput(t *testing.T) {
	for _, input := range []string{"process", "=SpringBoard", "bogus=x", "bogus!=x"} {
		if _, err := ParseLogFilters([]string{input}); err == nil {
			t.Errorf("expected error for filter %q, got none", input)
		}
	}
}

func TestMatchesFiltersRequiresEveryFilterToPass(t *testing.T) {
	entry := devices.LogEntry{PID: 42, Process: "SpringBoard", Level: "Error", Tag: "ActivityManager"}

	tests := []struct {
		name    string
		filters []string
		want    bool
	}{
		{"no filters matches everything", nil, true},
		{"single matching include", []string{"process=SpringBoard"}, true},
		{"single failing include", []string{"process=backboardd"}, false},
		{"matching exclude", []string{"process!=backboardd"}, true},
		{"failing exclude", []string{"process!=SpringBoard"}, false},
		{"all filters pass", []string{"level=Error", "process!=backboardd"}, true},
		{"one filter of many fails", []string{"level=Error", "tag=Zygote"}, false},
		{"numeric pid is compared as text", []string{"pid=42"}, true},
		{"empty field never matches a value", []string{"subsystem=com.apple.UIKit"}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filters, err := ParseLogFilters(test.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := matchesFilters(entry, filters); got != test.want {
				t.Errorf("matchesFilters(%v) = %v, want %v", test.filters, got, test.want)
			}
		})
	}
}

func assertFiltersEqual(t *testing.T, got, want []LogFilter) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d filters, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filter %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
