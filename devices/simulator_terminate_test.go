package devices

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

// simctlErrorExiting runs a shell that exits with the given status and returns
// the resulting *exec.ExitError wrapped the way runSimctl wraps it.
func simctlErrorExiting(t *testing.T, status int) error {
	t.Helper()
	err := exec.CommandContext(context.Background(), "sh", "-c", fmt.Sprintf("exit %d", status)).Run()
	if err == nil {
		t.Fatalf("expected the shell to exit with status %d", status)
	}
	return fmt.Errorf("failed to execute xcrun simctl command: %w", err)
}

func Test_isSimctlNothingToTerminate(t *testing.T) {
	if !isSimctlNothingToTerminate(simctlErrorExiting(t, 3)) {
		t.Fatal("exit status 3 must be recognised as 'found nothing to terminate'")
	}
	if isSimctlNothingToTerminate(simctlErrorExiting(t, 1)) {
		t.Fatal("exit status 1 is a real simctl failure, not 'nothing to terminate'")
	}
	if isSimctlNothingToTerminate(errors.New("failed to execute xcrun simctl command: exec: not found")) {
		t.Fatal("a non-exit error must not be treated as 'nothing to terminate'")
	}
}

func Test_terminateErrorForMissingProcess_MatchesRealDeviceMessages(t *testing.T) {
	if got := terminateErrorForMissingProcess("com.example.app", false).Error(); got != "com.example.app not installed" {
		t.Fatalf("not installed: unexpected message %q", got)
	}
	if got := terminateErrorForMissingProcess("com.example.app", true).Error(); got != "process of com.example.app not found" {
		t.Fatalf("installed but stopped: unexpected message %q", got)
	}
}
