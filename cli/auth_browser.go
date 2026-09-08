package cli

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
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

// verificationURLWithCode appends user_code so the login page can prefill it.
func verificationURLWithCode(verificationURI, userCode string) string {
	u, err := url.Parse(verificationURI)
	if err != nil {
		return verificationURI
	}
	q := u.Query()
	q.Set("user_code", userCode)
	u.RawQuery = q.Encode()
	return u.String()
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
	return nil
}
