package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/mobile-next/mobilecli/utils"
)

var noBrowser bool

// shouldSkipBrowser reports whether login must fall back to printing the
// device code instead of opening a browser: explicit --no-browser, an SSH
// session, CI, or a Linux box with no display.
func shouldSkipBrowser(noBrowserFlag bool, goos string, getenv func(string) string) bool {
	if noBrowserFlag {
		return true
	}
	if getenv("SSH_CONNECTION") != "" || getenv("SSH_TTY") != "" {
		return true
	}
	if getenv("CI") != "" {
		return true
	}
	// ponytail: macOS/Windows always have a display; only Linux/BSD can be headless
	if goos != "darwin" && goos != "windows" && getenv("DISPLAY") == "" && getenv("WAYLAND_DISPLAY") == "" {
		return true
	}
	return false
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}
	// ponytail: Start, not Run. xdg-open can block until the browser exits on some desktops.
	// Wait only reaps the child; the browser opening is what mattered and already happened.
	go func() {
		if waitErr := cmd.Wait(); waitErr != nil {
			utils.Verbose("browser opener exited: %v", waitErr)
		}
	}()
	return nil
}
