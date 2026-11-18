//go:build windows

package utils

import (
	"os/exec"
)

// ConfigureDetachedProcAttr is a no-op on Windows since process groups
// work differently. Context cancellation handles process termination.
func ConfigureDetachedProcAttr(cmd *exec.Cmd) {
	// No-op on Windows
}
