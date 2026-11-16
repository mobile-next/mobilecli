package devices

import (
	"strings"
	"testing"
)

// ps right-aligns the PID column, so a PID narrower than the column width is
// padded with leading spaces. This output is representative of a freshly booted
// machine, where PIDs are still low.
const psOutputPaddedPIDs = `  PID COMMAND
    1 /sbin/launchd
  324 /usr/libexec/logd
 1637 /Users/runner/Library/Developer/CoreSimulator/Devices/ABC-123/data/Containers/Bundle/Application/devicekit-iosUITests-Runner.app/devicekit-iosUITests-Runner DEVICEKIT_LISTEN_PORT=13180 HOME=/Users/runner
10418 /usr/sbin/cfprefsd
`

func TestParsePsOutputIncludesPaddedPIDs(t *testing.T) {
	processes := parsePsOutput(psOutputPaddedPIDs)

	if len(processes) != 4 {
		t.Fatalf("Expected 4 processes, got %d", len(processes))
	}

	expectedPIDs := []int{1, 324, 1637, 10418}
	for i, expected := range expectedPIDs {
		if processes[i].PID != expected {
			t.Errorf("Expected process %d to have PID %d, got %d", i, expected, processes[i].PID)
		}
	}
}

func TestParsePsOutputSkipsHeader(t *testing.T) {
	for _, proc := range parsePsOutput(psOutputPaddedPIDs) {
		if strings.HasPrefix(proc.Command, "COMMAND") {
			t.Errorf("Header line was parsed as a process: %+v", proc)
		}
	}
}

func TestParsePsOutputPreservesCommandAndEnvironment(t *testing.T) {
	processes := parsePsOutput(psOutputPaddedPIDs)

	var runner *ProcessInfo
	for i := range processes {
		if strings.Contains(processes[i].Command, "devicekit-iosUITests-Runner") {
			runner = &processes[i]
			break
		}
	}

	if runner == nil {
		t.Fatal("Expected to find the devicekit runner process")
	}

	if runner.PID != 1637 {
		t.Errorf("Expected runner PID 1637, got %d", runner.PID)
	}

	if !strings.HasPrefix(runner.Command, "/Users/runner/Library") {
		t.Errorf("Expected command to start at the executable path, got %q", runner.Command)
	}

	// the command field carries the environment that getDeviceKitEnvPort reads back
	port, err := extractEnvValue(runner.Command, "DEVICEKIT_LISTEN_PORT")
	if err != nil {
		t.Fatalf("Unexpected error extracting port: %v", err)
	}

	if port != "13180" {
		t.Errorf("Expected port 13180, got %s", port)
	}
}

// findDeviceKitProcessForDevice must locate a running agent regardless of how
// wide its PID happens to be, otherwise StartAgent relaunches the agent on a new
// port, times out waiting for it, and terminates the healthy agent.
func TestParsePsOutputFindsAgentByDevicePath(t *testing.T) {
	const deviceUDID = "ABC-123"
	devicePath := "/Library/Developer/CoreSimulator/Devices/" + deviceUDID

	found := false
	for _, proc := range parsePsOutput(psOutputPaddedPIDs) {
		if strings.Contains(proc.Command, devicePath) && strings.Contains(proc.Command, "devicekit-iosUITests-Runner") {
			found = true
		}
	}

	if !found {
		t.Error("Expected to find the running agent process for the device")
	}
}

func TestParsePsOutputHandlesMalformedLines(t *testing.T) {
	output := `  PID COMMAND
    1 /sbin/launchd

notanumber /some/command
   42
  777 /usr/bin/foo
`

	processes := parsePsOutput(output)

	if len(processes) != 2 {
		t.Fatalf("Expected 2 processes, got %d: %+v", len(processes), processes)
	}

	if processes[0].PID != 1 || processes[1].PID != 777 {
		t.Errorf("Expected PIDs 1 and 777, got %d and %d", processes[0].PID, processes[1].PID)
	}
}
