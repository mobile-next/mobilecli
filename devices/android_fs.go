package devices

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"al.essio.dev/pkg/shellescape"
	"github.com/google/uuid"
)

// androidPackageName extracts the package name from a /data/user/<uid>/<package>/... path.
func androidPackageName(remotePath string) (string, error) {
	parts := strings.SplitN(remotePath, "/", 6)
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid /data/user/ path: %s", remotePath)
	}
	return parts[4], nil
}

// buildShellCommand returns a single shell-safe command string for `adb shell` or
// `adb exec-out`. every token is quoted so package names and paths containing spaces
// or shell metacharacters cannot escape into the device shell. paths under /data/user/
// are wrapped with `run-as <pkg>`.
func (d *AndroidDevice) buildShellCommand(remotePath string, parts ...string) (string, error) {
	cmd := shellescape.QuoteCommand(parts)
	if !strings.HasPrefix(remotePath, "/data/user/") {
		return cmd, nil
	}
	pkg, err := androidPackageName(remotePath)
	if err != nil {
		return "", err
	}
	return "run-as " + shellescape.Quote(pkg) + " " + cmd, nil
}

func (d *AndroidDevice) PushFile(localPath, remotePath string) error {
	if !strings.HasPrefix(remotePath, "/data/user/") {
		_, err := d.runAdbCommand("push", localPath, remotePath)
		return err
	}

	tmpPath := fmt.Sprintf("/data/local/tmp/mobilecli-%s", uuid.NewString())
	if _, err := d.runAdbCommand("push", localPath, tmpPath); err != nil {
		return fmt.Errorf("push to tmp failed: %w", err)
	}

	cpCmd, err := d.buildShellCommand(remotePath, "cp", tmpPath, remotePath)
	if err != nil {
		return err
	}
	_, cpErr := d.runAdbCommand("shell", cpCmd)

	rmCmd := shellescape.QuoteCommand([]string{"rm", tmpPath})
	_, rmErr := d.runAdbCommand("shell", rmCmd)

	if cpErr != nil {
		return fmt.Errorf("copy to app container failed: %w", cpErr)
	}
	if rmErr != nil {
		return fmt.Errorf("cleanup of tmp file failed: %w", rmErr)
	}
	return nil
}

func (d *AndroidDevice) PullFile(remotePath, localPath string) error {
	shellCmd, err := d.buildShellCommand(remotePath, "cat", remotePath)
	if err != nil {
		return err
	}

	// exec-out (instead of shell) bypasses the PTY, preserving binary bytes on Windows
	// and keeping stderr separate so we can surface it on failure
	deviceID := d.getAdbIdentifier()
	cmd := exec.Command(getAdbPath(), "-s", deviceID, "exec-out", shellCmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("pull failed: %w: %s", err, msg)
		}
		return fmt.Errorf("pull failed: %w", err)
	}
	return os.WriteFile(localPath, data, 0644)
}

func (d *AndroidDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	if remotePath == "" {
		remotePath = "/"
	}

	// LANG=C pins the date format to YYYY-MM-DD HH:MM regardless of device locale.
	// trailing slash makes ls follow symlinks (e.g. /sdcard -> /storage/self/primary);
	// fall back to the original path if that fails (path is a file, not a dir).
	lsPath := strings.TrimRight(remotePath, "/") + "/"
	lsCmd, err := d.buildShellCommand(remotePath, "ls", "-la", lsPath)
	if err != nil {
		return nil, err
	}
	output, err := d.runAdbCommand("shell", "LANG=C "+lsCmd)
	if err != nil {
		lsCmd, err = d.buildShellCommand(remotePath, "ls", "-la", remotePath)
		if err != nil {
			return nil, err
		}
		output, err = d.runAdbCommand("shell", "LANG=C "+lsCmd)
	}
	if err != nil {
		return nil, fmt.Errorf("ls failed: %w", err)
	}

	return androidParseLsOutput(string(output), remotePath), nil
}

// toybox `ls -la` line under LANG=C:
//
//	<perms> <links> <owner> <group> <size|major,minor> <YYYY-MM-DD> <HH:MM> <name>[ -> <target>]
//
// the size column is numeric for files; character/block devices show `<major>, <minor>`
// (with optional spaces around the comma).
var androidLsLineRe = regexp.MustCompile(
	`^([-dlbcps])\S*\s+\d+\s+\S+\s+\S+\s+(?:\d+,\s*\d+|\d+)\s+(\d{4}-\d{2}-\d{2})\s+(\d{2}:\d{2})\s+(.+)$`,
)

func androidParseLsOutput(output, dirPath string) []FileEntry {
	dirPath = strings.TrimRight(dirPath, "/")
	entries := []FileEntry{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		entry := androidParseLsLine(line, dirPath)
		if entry == nil || entry.Name == "." || entry.Name == ".." {
			continue
		}
		entries = append(entries, *entry)
	}
	return entries
}

func androidParseLsLine(line, dirPath string) *FileEntry {
	m := androidLsLineRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	typeChar := m[1]
	isDir := typeChar == "d"

	// re-extract numeric size from the original tokens; we've already validated layout
	var size int64
	if !isDir {
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			size, _ = strconv.ParseInt(fields[4], 10, 64)
		}
	}

	modTime, _ := time.Parse("2006-01-02 15:04", m[2]+" "+m[3])

	name := m[4]
	if typeChar == "l" {
		if idx := strings.Index(name, " -> "); idx != -1 {
			name = name[:idx]
		}
	}

	// when listing a single file, ls echoes the absolute path as the name
	var entryPath string
	if strings.HasPrefix(name, "/") {
		entryPath = name
		name = name[strings.LastIndex(name, "/")+1:]
	} else {
		entryPath = strings.TrimRight(dirPath, "/") + "/" + name
	}

	return &FileEntry{
		Name:    name,
		Path:    entryPath,
		Size:    size,
		ModTime: modTime,
		IsDir:   isDir,
	}
}

func (d *AndroidDevice) Mkdir(bundleID, remotePath string, parents bool) error {
	parts := []string{"mkdir"}
	if parents {
		parts = append(parts, "-p")
	}
	parts = append(parts, remotePath)
	cmd, err := d.buildShellCommand(remotePath, parts...)
	if err != nil {
		return err
	}
	_, err = d.runAdbCommand("shell", cmd)
	return err
}

func (d *AndroidDevice) Rm(bundleID, remotePath string, recursive bool) error {
	parts := []string{"rm"}
	if recursive {
		parts = append(parts, "-rf")
	}
	parts = append(parts, remotePath)
	cmd, err := d.buildShellCommand(remotePath, parts...)
	if err != nil {
		return err
	}
	_, err = d.runAdbCommand("shell", cmd)
	return err
}
