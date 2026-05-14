package devices

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

// runAsArgs returns the adb shell prefix for the given path.
// App container paths under /data/user/ are prefixed with run-as <package>.
func (d *AndroidDevice) runAsArgs(remotePath string) ([]string, error) {
	if !strings.HasPrefix(remotePath, "/data/user/") {
		return []string{"shell"}, nil
	}
	pkg, err := androidPackageName(remotePath)
	if err != nil {
		return nil, err
	}
	return []string{"shell", "run-as", pkg}, nil
}

func (d *AndroidDevice) PushFile(localPath, remotePath string) error {
	if !strings.HasPrefix(remotePath, "/data/user/") {
		_, err := d.runAdbCommand("push", localPath, remotePath)
		return err
	}

	shellArgs, err := d.runAsArgs(remotePath)
	if err != nil {
		return err
	}

	tmpPath := fmt.Sprintf("/data/local/tmp/mobilecli-%s", uuid.NewString())
	if _, err := d.runAdbCommand("push", localPath, tmpPath); err != nil {
		return fmt.Errorf("push to tmp failed: %w", err)
	}

	_, cpErr := d.runAdbCommand(append(shellArgs, "cp", tmpPath, remotePath)...)
	_, rmErr := d.runAdbCommand("shell", "rm", tmpPath)

	if cpErr != nil {
		return fmt.Errorf("copy to app container failed: %w", cpErr)
	}
	if rmErr != nil {
		return fmt.Errorf("cleanup of tmp file failed: %w", rmErr)
	}

	return nil
}

func (d *AndroidDevice) PullFile(remotePath, localPath string) error {
	shellArgs, err := d.runAsArgs(remotePath)
	if err != nil {
		return err
	}
	// exec-out instead of shell avoids PTY CRLF translation on Windows
	// replace "shell" with "exec-out" so the rest of the args are forwarded as-is
	pullArgs := append([]string{"exec-out"}, shellArgs[1:]...)
	data, err := d.runAdbCommandStdout(append(pullArgs, "cat", remotePath)...)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	return os.WriteFile(localPath, data, 0644)
}

func (d *AndroidDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	if remotePath == "" {
		remotePath = "/"
	}

	shellArgs, err := d.runAsArgs(remotePath)
	if err != nil {
		return nil, err
	}

	// append trailing slash so symlinks (e.g. /sdcard) are followed;
	// fall back to the original path if that fails (path is a file, not a dir)
	lsPath := strings.TrimRight(remotePath, "/") + "/"
	output, err := d.runAdbCommand(append(shellArgs, "ls", "-la", lsPath)...)
	if err != nil {
		output, err = d.runAdbCommand(append(shellArgs, "ls", "-la", remotePath)...)
	}

	if err != nil {
		return nil, fmt.Errorf("ls failed: %w", err)
	}

	return androidParseLsOutput(string(output), remotePath), nil
}

func androidParseLsOutput(output, dirPath string) []FileEntry {
	dirPath = strings.TrimRight(dirPath, "/")
	var entries []FileEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		entry := androidParseLsLine(line, dirPath)
		if entry == nil || entry.Name == "." || entry.Name == ".." {
			continue
		}
		entries = append(entries, *entry)
	}
	if entries == nil {
		entries = []FileEntry{}
	}
	return entries
}

// androidParseLsLine parses a single line of Android ls -la output.
// Expected format: <perms> <links> <owner> <group> <size> <date> <time> <name>
func androidParseLsLine(line, dirPath string) *FileEntry {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil
	}

	perms := fields[0]
	isDir := strings.HasPrefix(perms, "d")

	var size int64
	if !isDir {
		size, _ = strconv.ParseInt(fields[4], 10, 64)
	}

	modTime, _ := time.Parse("2006-01-02 15:04", fields[5]+" "+fields[6])

	name := strings.Join(fields[7:], " ")
	// strip symlink target if present
	if idx := strings.Index(name, " -> "); idx != -1 {
		name = name[:idx]
	}

	// when listing a single file, ls emits the absolute path as the name
	var entryPath string
	if strings.HasPrefix(name, "/") {
		entryPath = name
		name = name[strings.LastIndex(name, "/")+1:]
	} else {
		entryPath = dirPath + "/" + name
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
	shellArgs, err := d.runAsArgs(remotePath)
	if err != nil {
		return err
	}
	mkdirArgs := []string{"mkdir"}
	if parents {
		mkdirArgs = append(mkdirArgs, "-p")
	}
	mkdirArgs = append(mkdirArgs, remotePath)
	_, err = d.runAdbCommand(append(shellArgs, mkdirArgs...)...)
	return err
}

func (d *AndroidDevice) Rm(bundleID, remotePath string, recursive bool) error {
	shellArgs, err := d.runAsArgs(remotePath)
	if err != nil {
		return err
	}
	rmArgs := []string{"rm"}
	if recursive {
		rmArgs = append(rmArgs, "-rf")
	}
	rmArgs = append(rmArgs, remotePath)
	_, err = d.runAdbCommand(append(shellArgs, rmArgs...)...)
	return err
}
