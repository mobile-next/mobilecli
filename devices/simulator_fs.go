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
func (s *SimulatorDevice) validatePath(path string) error {
	root, err := s.simulatorDeviceRoot()
	if err != nil {
		return err
	}
	clean := filepath.Clean(path)
	if clean != root && !strings.HasPrefix(clean, root+string(filepath.Separator)) {
		return fmt.Errorf("path '%s' is outside the simulator device directory", path)
	}
	return nil
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
	return os.WriteFile(localPath, data, 0644)
}

func (s *SimulatorDevice) PushFile(localPath, remotePath string) error {
	if err := s.validatePath(remotePath); err != nil {
		return err
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read local file failed: %w", err)
	}
	return os.WriteFile(remotePath, data, 0644)
}

func (s *SimulatorDevice) Mkdir(bundleID, remotePath string, parents bool) error {
	if err := s.validatePath(remotePath); err != nil {
		return err
	}
	if parents {
		return os.MkdirAll(remotePath, 0755)
	}
	return os.Mkdir(remotePath, 0755)
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
