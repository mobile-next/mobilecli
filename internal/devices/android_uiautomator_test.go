package devices

import (
	"errors"
	"strings"
	"testing"
)

const hierarchy = `<?xml version='1.0' encoding='UTF-8' standalone='yes' ?><hierarchy rotation="0"></hierarchy>`

// scriptedDump returns each output in turn, as successive uiautomator runs would.
func scriptedDump(outputs ...string) (func() ([]byte, error), *int) {
	calls := 0
	return func() ([]byte, error) {
		output := outputs[len(outputs)-1]
		if calls < len(outputs) {
			output = outputs[calls]
		}
		calls++
		return []byte(output), nil
	}, &calls
}

func TestReadUiAutomatorDumpTriesAgainWhenADumpHasNoXML(t *testing.T) {
	dump, calls := scriptedDump("ERROR: could not get idle state.\n", hierarchy)

	got, err := readUiAutomatorDump(dump)

	if err != nil || got != hierarchy {
		t.Fatalf("got %q, %v; want the second dump's hierarchy", got, err)
	}
	if *calls != 2 {
		t.Fatalf("dumped %d times, want 2", *calls)
	}
}

func TestReadUiAutomatorDumpTriesAgainOnANullRootNode(t *testing.T) {
	dump, _ := scriptedDump("null root node returned by UiTestAutomationBridge.\n", hierarchy)

	got, err := readUiAutomatorDump(dump)

	if err != nil || got != hierarchy {
		t.Fatalf("got %q, %v; want the second dump's hierarchy", got, err)
	}
}

func TestReadUiAutomatorDumpDropsTheStatusLineBeforeTheXML(t *testing.T) {
	dump, _ := scriptedDump("UI hierchary dumped to: /dev/tty\n" + hierarchy)

	got, err := readUiAutomatorDump(dump)

	if err != nil || got != hierarchy {
		t.Fatalf("got %q, %v; want the hierarchy alone", got, err)
	}
}

func TestReadUiAutomatorDumpGivesUpAfterTheLastTryWithoutXML(t *testing.T) {
	dump, calls := scriptedDump("")

	_, err := readUiAutomatorDump(dump)

	if err == nil || !strings.Contains(err.Error(), "no XML content found in uiautomator dump") {
		t.Fatalf("got %v, want the no-XML error", err)
	}
	if *calls != uiAutomatorDumpTries {
		t.Fatalf("dumped %d times, want %d", *calls, uiAutomatorDumpTries)
	}
}

func TestReadUiAutomatorDumpReportsANullRootNodeOnTheLastTry(t *testing.T) {
	dump, _ := scriptedDump("null root node returned by UiTestAutomationBridge.\n")

	_, err := readUiAutomatorDump(dump)

	if err == nil || !strings.Contains(err.Error(), "failed to get UIAutomator XML") {
		t.Fatalf("got %v, want the null-root error", err)
	}
}

func TestReadUiAutomatorDumpStopsWhenAdbFails(t *testing.T) {
	calls := 0
	adbGone := errors.New("device offline")

	_, err := readUiAutomatorDump(func() ([]byte, error) {
		calls++
		return nil, adbGone
	})

	if !errors.Is(err, adbGone) || calls != 1 {
		t.Fatalf("got %v after %d dumps, want the adb error after 1", err, calls)
	}
}
