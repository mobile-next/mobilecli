package devices

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func (d *AndroidDevice) PushFile(bundleID, localPath, remotePath string) error {
	return errors.New("not implemented")
}

func (d *AndroidDevice) PullFile(bundleID, remotePath, localPath string) error {
	var data []byte
	var err error

	if strings.HasPrefix(remotePath, "/data/user/") {
		parts := strings.SplitN(remotePath, "/", 6)
		if len(parts) < 5 {
			return fmt.Errorf("invalid /data/user/ path: %s", remotePath)
		}
		packageName := parts[4]
		data, err = d.runAdbCommandStdout("shell", "run-as", packageName, "cat", remotePath)
	} else {
		data, err = d.runAdbCommandStdout("exec-out", "cat", remotePath)
	}

	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	return os.WriteFile(localPath, data, 0644)
}

func (d *AndroidDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	var output []byte
	var err error

	// append trailing slash so symlinks (e.g. /sdcard) are followed;
	// fall back to the original path if that fails (path is a file, not a dir)
	lsPath := strings.TrimRight(remotePath, "/") + "/"

	if strings.HasPrefix(remotePath, "/data/user/") {
		// path structure: /data/user/<uid>/<package>/...
		// extract package name as the 5th segment
		parts := strings.SplitN(remotePath, "/", 6)
		if len(parts) < 5 {
			return nil, fmt.Errorf("invalid /data/user/ path: %s", remotePath)
		}
		packageName := parts[4]
		output, err = d.runAdbCommand("shell", "run-as", packageName, "ls", "-la", lsPath)
		if err != nil {
			output, err = d.runAdbCommand("shell", "run-as", packageName, "ls", "-la", remotePath)
		}
	} else {
		output, err = d.runAdbCommand("shell", "ls", "-la", lsPath)
		if err != nil {
			output, err = d.runAdbCommand("shell", "ls", "-la", remotePath)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("ls failed: %w", err)
	}

	return parseLsOutput(string(output), remotePath), nil
}

func parseLsOutput(output, dirPath string) []FileEntry {
	dirPath = strings.TrimRight(dirPath, "/")
	var entries []FileEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		entry := parseLsLine(line, dirPath)
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

// parseLsLine parses a single line of Android ls -la output.
// Expected format: <perms> <links> <owner> <group> <size> <date> <time> <name>
func parseLsLine(line, dirPath string) *FileEntry {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return nil
	}

	perms := fields[0]
	isDir := strings.HasPrefix(perms, "d")

	size, _ := strconv.ParseInt(fields[4], 10, 64)

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

func (d *AndroidDevice) Mkdir(bundleID, remotePath string) error {
	return errors.New("not implemented")
}

func (d *AndroidDevice) Rm(bundleID, remotePath string) error {
	return errors.New("not implemented")
}
