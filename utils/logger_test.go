package utils

import "testing"

func TestSetVerbose_And_IsVerbose(t *testing.T) {
	// save original state and restore after test
	original := IsVerbose()
	defer SetVerbose(original)

	SetVerbose(true)
	if !IsVerbose() {
		t.Error("expected IsVerbose() = true after SetVerbose(true)")
	}

	SetVerbose(false)
	if IsVerbose() {
		t.Error("expected IsVerbose() = false after SetVerbose(false)")
	}
}

func TestVerbose_DoesNotPanicWhenDisabled(t *testing.T) {
	original := IsVerbose()
	defer SetVerbose(original)

	SetVerbose(false)
	// should not panic
	Verbose("test message %s %d", "arg", 42)
}

func TestVerbose_DoesNotPanicWhenEnabled(t *testing.T) {
	original := IsVerbose()
	defer SetVerbose(original)

	SetVerbose(true)
	// should not panic
	Verbose("test message %s %d", "arg", 42)
}

func TestInfo_DoesNotPanic(t *testing.T) {
	// should not panic
	Info("test info %s", "message")
}
