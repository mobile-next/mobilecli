package devices

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// simulatorDeviceRoot returns the CoreSimulator device directory for this simulator.
// All allowed paths must be within this root to prevent accidental Mac filesystem access.
func (s *SimulatorDevice) simulatorDeviceRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices", s.UDID), nil
}

// validatePath ensures the given path is within the simulator's device directory.
//
// A simulator's "device" filesystem is just directories on the Mac, so every fs
// operation is a host os.* call. validatePath is the only boundary stopping those
// calls from reaching the rest of the Mac, which makes it security-sensitive: a
// lexical prefix check is not enough, because a symlink inside the sandbox can
// point outside it and os.* follows it. We therefore resolve symlinks before
// comparing, so neither a symlinked leaf (an existing fs push/pull target) nor a
// symlinked parent (e.g. Documents/x -> /etc used by fs ls/pull/push) can escape.
func (s *SimulatorDevice) validatePath(path string) error {
	root, err := s.simulatorDeviceRoot()
	if err != nil {
		return err
	}
	return validatePathWithinRoot(root, path)
}

// validatePathWithinRoot reports whether path resolves to a location inside root,
// resolving symlinks in both so the check reflects what the OS will actually open
// rather than the textual path. It is a package-level function with no device
// state so the boundary can be unit-tested against real temp directories.
func validatePathWithinRoot(root, path string) error {
	// Canonicalize the root. It normally exists; if it cannot be resolved (an
	// offline or unknown device) fall back to the cleaned path so legitimate
	// paths under it are still accepted.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = filepath.Clean(root)
	}

	resolved, err := resolveWithinExisting(path)
	if err != nil {
		return fmt.Errorf("path '%s' is outside the simulator device directory", path)
	}

	if resolved != resolvedRoot && !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) {
		return fmt.Errorf("path '%s' is outside the simulator device directory", path)
	}
	return nil
}

// resolveWithinExisting returns the canonical absolute form of p, resolving any
// symlinks in the portion of p that already exists on disk. The trailing
// components that do not exist yet (as when fs push or fs mkdir creates them) are
// cleaned and re-appended to the resolved ancestor. A symlink can only be
// traversed where it exists, so resolving the longest existing ancestor is enough
// to catch an escape through either the leaf or any parent, while still allowing
// not-yet-created targets.
func resolveWithinExisting(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)

	var missing []string
	cur := abs
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Nothing along the path exists; no symlink can have been traversed,
			// so the cleaned absolute path is already canonical.
			return abs, nil
		}
		missing = append(missing, filepath.Base(cur))
		cur = parent
	}
}

func (s *SimulatorDevice) GetAppContainerPath(bundleID string) (string, error) {
	output, err := runSimctl("get_app_container", s.UDID, bundleID, "data")
	if err != nil {
		return "", fmt.Errorf("get_app_container failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *SimulatorDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	if remotePath == "" {
		if bundleID != "" {
			var err error
			remotePath, err = s.GetAppContainerPath(bundleID)
			if err != nil {
				return nil, err
			}
		} else {
			root, err := s.simulatorDeviceRoot()
			if err != nil {
				return nil, err
			}
			remotePath = filepath.Join(root, "data")
		}
	}

	if err := s.validatePath(remotePath); err != nil {
		return nil, err
	}

	dirEntries, err := os.ReadDir(remotePath)
	if err != nil {
		// path might be a single file
		info, statErr := os.Stat(remotePath)
		if statErr != nil {
			return nil, fmt.Errorf("ls failed: %w", err)
		}
		return []FileEntry{{
			Name:    filepath.Base(remotePath),
			Path:    remotePath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   false,
		}}, nil
	}

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		size := info.Size()
		if de.IsDir() {
			size = 0
		}
		entries = append(entries, FileEntry{
			Name:    de.Name(),
			Path:    filepath.Join(remotePath, de.Name()),
			Size:    size,
			ModTime: info.ModTime(),
			IsDir:   de.IsDir(),
		})
	}
	return entries, nil
}

func (s *SimulatorDevice) PullFile(remotePath, localPath string) error {
	if err := s.validatePath(remotePath); err != nil {
		return err
	}
	data, err := os.ReadFile(remotePath)
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}
	return os.WriteFile(localPath, data, 0600)
}

func (s *SimulatorDevice) PushFile(localPath, remotePath string) error {
	if err := s.validatePath(remotePath); err != nil {
		return err
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file failed: %w", err)
	}
	return os.WriteFile(remotePath, data, 0600)
}

func (s *SimulatorDevice) Mkdir(bundleID, remotePath string, parents bool) error {
	if err := s.validatePath(remotePath); err != nil {
		return err
	}
	if parents {
		return os.MkdirAll(remotePath, 0750)
	}
	return os.Mkdir(remotePath, 0750)
}

func (s *SimulatorDevice) Rm(bundleID, remotePath string, recursive bool) error {
	if err := s.validatePath(remotePath); err != nil {
		return err
	}
	if recursive {
		return os.RemoveAll(remotePath)
	}
	return os.Remove(remotePath)
}
