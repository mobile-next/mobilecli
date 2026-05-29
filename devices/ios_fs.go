package devices

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/house_arrest"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
)

const iosAppContainerPrefix = "/private/var/mobile/Containers/Data/Application/"

// iosBrowseAllApps returns all installed apps via the installation proxy.
func (d *IOSDevice) iosBrowseAllApps() ([]installationproxy.AppInfo, error) {
	device, err := d.getEnhancedDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}
	svc, err := installationproxy.New(device)
	if err != nil {
		return nil, fmt.Errorf("installationproxy failed: %w", err)
	}
	defer svc.Close()
	return svc.BrowseAllApps()
}

// resolveAfcClientAndPath returns the right AFC client and normalized path.
// App container absolute paths are transparently routed through House Arrest.
func (d *IOSDevice) resolveAfcClientAndPath(bundleID, remotePath string) (*afc.Client, string, error) {
	device, err := d.getEnhancedDevice()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get device: %w", err)
	}

	if bundleID != "" {
		client, err := house_arrest.New(device, bundleID)
		return client, remotePath, err
	}

	// detect absolute app container path and route through House Arrest
	if strings.HasPrefix(remotePath, iosAppContainerPrefix) {
		rest := strings.TrimPrefix(remotePath, iosAppContainerPrefix)
		parts := strings.SplitN(rest, "/", 2)
		uuid := parts[0]
		containerRelPath := "/"
		if len(parts) == 2 && parts[1] != "" {
			containerRelPath = "/" + parts[1]
		}

		resolvedBundleID, err := d.bundleIDForContainerUUID(uuid)
		if err != nil {
			return nil, "", fmt.Errorf("cannot access app container (use bundle-id instead): %w", err)
		}
		client, err := house_arrest.New(device, resolvedBundleID)
		return client, containerRelPath, err
	}

	client, err := afc.New(device)
	return client, remotePath, err
}

// bundleIDForContainerUUID looks up which app owns a given data container UUID.
func (d *IOSDevice) bundleIDForContainerUUID(uuid string) (string, error) {
	apps, err := d.iosBrowseAllApps()
	if err != nil {
		return "", err
	}
	for _, app := range apps {
		container, ok := app["Container"].(string)
		if !ok {
			continue
		}
		if strings.HasSuffix(strings.TrimRight(container, "/"), uuid) {
			return app.CFBundleIdentifier(), nil
		}
	}
	return "", fmt.Errorf("no app found with container UUID %s", uuid)
}

func (d *IOSDevice) GetAppContainerPath(bundleID string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.startTunnel(); err != nil {
		return "", fmt.Errorf("failed to start tunnel: %w", err)
	}

	apps, err := d.iosBrowseAllApps()
	if err != nil {
		return "", err
	}

	for _, app := range apps {
		if app.CFBundleIdentifier() == bundleID {
			if container, ok := app["Container"].(string); ok {
				return container, nil
			}
			return "", fmt.Errorf("app %s has no data container (system app?)", bundleID)
		}
	}

	return "", fmt.Errorf("app %s not found on device", bundleID)
}

func (d *IOSDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	if remotePath == "" {
		remotePath = "/"
	}

	if err := func() error {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.startTunnel()
	}(); err != nil {
		return nil, fmt.Errorf("failed to start tunnel: %w", err)
	}

	client, remotePath, err := d.resolveAfcClientAndPath(bundleID, remotePath)
	if err != nil {
		return nil, fmt.Errorf("afc connect failed: %w", err)
	}
	defer client.Close()

	names, err := client.List(remotePath)
	if err != nil {
		// path might be a single file — stat it directly
		info, statErr := client.Stat(remotePath)
		if statErr != nil {
			return nil, fmt.Errorf("ls failed: %w", err)
		}
		size := int64(0)
		if !info.IsDir() {
			size = info.Size
		}
		return []FileEntry{{
			Name:  path.Base(remotePath),
			Path:  remotePath,
			Size:  size,
			IsDir: info.IsDir(),
		}}, nil
	}

	entries := make([]FileEntry, 0, len(names))
	for _, name := range names {
		fullPath := strings.TrimRight(remotePath, "/") + "/" + name
		entry := FileEntry{Name: name, Path: fullPath}
		if info, err := client.Stat(fullPath); err == nil {
			entry.IsDir = info.IsDir()
			if !entry.IsDir {
				entry.Size = info.Size
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (d *IOSDevice) PushFile(localPath, remotePath string) error {
	return errors.New("not implemented")
}

func (d *IOSDevice) PullFile(remotePath, localPath string) error {
	return errors.New("not implemented")
}

func (d *IOSDevice) Mkdir(bundleID, remotePath string, parents bool) error {
	return errors.New("not implemented")
}

func (d *IOSDevice) Rm(bundleID, remotePath string, recursive bool) error {
	return errors.New("not implemented")
}
