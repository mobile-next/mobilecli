package devices

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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
	defer func() { _ = client.Close() }()

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

// withAfc starts the tunnel, resolves the AFC client for the path and runs fn
// with the client adapter and the resolved (container-relative) path.
func (d *IOSDevice) withAfc(bundleID, remotePath string, fn func(fs afcFS, resolved string) error) error {
	if err := func() error {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.startTunnel()
	}(); err != nil {
		return fmt.Errorf("failed to start tunnel: %w", err)
	}

	client, resolved, err := d.resolveAfcClientAndPath(bundleID, remotePath)
	if err != nil {
		return fmt.Errorf("afc connect failed: %w", err)
	}
	defer func() { _ = client.Close() }()
	return fn(afcClientFS{c: client}, resolved)
}

func (d *IOSDevice) PushFile(localPath, remotePath string) error {
	return d.withAfc("", remotePath, func(fs afcFS, resolved string) error {
		return afcPush(fs, localPath, resolved)
	})
}

func (d *IOSDevice) PullFile(remotePath, localPath string) error {
	return d.withAfc("", remotePath, func(fs afcFS, resolved string) error {
		return afcPull(fs, resolved, localPath)
	})
}

func (d *IOSDevice) Mkdir(bundleID, remotePath string, parents bool) error {
	return d.withAfc(bundleID, remotePath, func(fs afcFS, resolved string) error {
		return afcMkdir(fs, resolved, parents)
	})
}

func (d *IOSDevice) Rm(bundleID, remotePath string, recursive bool) error {
	return d.withAfc(bundleID, remotePath, func(fs afcFS, resolved string) error {
		return afcRm(fs, resolved, recursive)
	})
}

// afcChunkSize bounds a single AFC write packet.
const afcChunkSize = 32 * 1024

// afcFS is the subset of *afc.Client used by the file operations, so the logic
// can be unit-tested without a device.
type afcFS interface {
	Stat(p string) (afc.FileInfo, error)
	OpenRead(p string) (io.ReadCloser, error)
	OpenWrite(p string) (io.WriteCloser, error)
	MkDir(p string) error
	Remove(p string) error
	RemoveAll(p string) error
}

// afcClientFS adapts *afc.Client to afcFS.
type afcClientFS struct{ c *afc.Client }

func (a afcClientFS) Stat(p string) (afc.FileInfo, error) { return a.c.Stat(p) }
func (a afcClientFS) OpenRead(p string) (io.ReadCloser, error) {
	return a.c.Open(p, afc.READ_ONLY)
}
func (a afcClientFS) OpenWrite(p string) (io.WriteCloser, error) {
	return a.c.Open(p, afc.WRITE_ONLY_CREATE_TRUNC)
}
func (a afcClientFS) MkDir(p string) error     { return a.c.MkDir(p) }
func (a afcClientFS) Remove(p string) error    { return a.c.Remove(p) }
func (a afcClientFS) RemoveAll(p string) error { return a.c.RemoveAll(p) }

// cleanAfcPath normalizes a device path and rejects any escape above the root.
// The result is absolute and slash-separated.
func cleanAfcPath(p string) (string, error) {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("path %q must not contain '..'", p)
		}
	}
	return path.Clean("/" + p), nil
}

func afcExists(fs afcFS, p string) (afc.FileInfo, bool) {
	info, err := fs.Stat(p)
	return info, err == nil
}

// afcPush copies the local regular file to remote. If remote is an existing
// directory the local base name is appended. The parent directory must exist.
func afcPush(fs afcFS, localPath, remotePath string) error {
	st, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("local file: %w", err)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("local path %s is not a regular file", localPath)
	}
	remote, err := cleanAfcPath(remotePath)
	if err != nil {
		return err
	}
	if info, ok := afcExists(fs, remote); ok && info.IsDir() {
		remote = path.Join(remote, filepath.Base(localPath))
	}
	if remote == "/" {
		return errors.New("remote path must name a file")
	}
	if _, ok := afcExists(fs, path.Dir(remote)); !ok {
		return fmt.Errorf("remote parent directory %s does not exist", path.Dir(remote))
	}

	src, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := fs.OpenWrite(remote)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	// Wrap both ends so io.CopyBuffer uses our bounded buffer (no ReaderFrom/WriterTo shortcut).
	_, copyErr := io.CopyBuffer(struct{ io.Writer }{dst}, struct{ io.Reader }{src}, make([]byte, afcChunkSize))
	closeErr := dst.Close()
	if copyErr != nil {
		return fmt.Errorf("write remote file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close remote file: %w", closeErr)
	}
	info, err := fs.Stat(remote)
	if err != nil {
		return fmt.Errorf("verify remote file: %w", err)
	}
	if info.Size != st.Size() {
		return fmt.Errorf("remote size %d does not match local size %d", info.Size, st.Size())
	}
	return nil
}

// afcPull copies the remote regular file to local. If local is an existing
// directory the remote base name is appended. The file is written to a temp
// file and renamed on success, so no partial file is left behind.
func afcPull(fs afcFS, remotePath, localPath string) error {
	remote, err := cleanAfcPath(remotePath)
	if err != nil {
		return err
	}
	info, err := fs.Stat(remote)
	if err != nil {
		return fmt.Errorf("remote file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("remote path %s is a directory", remote)
	}
	if st, err := os.Stat(localPath); err == nil && st.IsDir() {
		localPath = filepath.Join(localPath, path.Base(remote))
	}

	src, err := fs.OpenRead(remote)
	if err != nil {
		return fmt.Errorf("open remote file: %w", err)
	}
	defer func() { _ = src.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(localPath), ".afc-pull-*")
	if err != nil {
		return fmt.Errorf("create local temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	n, copyErr := io.CopyBuffer(struct{ io.Writer }{tmp}, struct{ io.Reader }{src}, make([]byte, afcChunkSize))
	closeErr := tmp.Close()
	if copyErr != nil {
		cleanup()
		return fmt.Errorf("read remote file: %w", copyErr)
	}
	if closeErr != nil {
		cleanup()
		return fmt.Errorf("write local file: %w", closeErr)
	}
	if n != info.Size {
		cleanup()
		return fmt.Errorf("pulled %d bytes, remote size is %d", n, info.Size)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		cleanup()
		return fmt.Errorf("chmod local file: %w", err)
	}
	if err := os.Rename(tmpName, localPath); err != nil {
		cleanup()
		return fmt.Errorf("move local file into place: %w", err)
	}
	return nil
}

// afcMkdir creates a directory; with parents it creates every missing level.
func afcMkdir(fs afcFS, remotePath string, parents bool) error {
	remote, err := cleanAfcPath(remotePath)
	if err != nil {
		return err
	}
	if remote == "/" {
		return errors.New("cannot create the root directory")
	}
	if !parents {
		return fs.MkDir(remote)
	}
	cur := ""
	for _, seg := range strings.Split(strings.TrimPrefix(remote, "/"), "/") {
		cur += "/" + seg
		info, ok := afcExists(fs, cur)
		if ok {
			if !info.IsDir() {
				return fmt.Errorf("%s exists and is not a directory", cur)
			}
			continue
		}
		if err := fs.MkDir(cur); err != nil {
			return err
		}
	}
	return nil
}

// afcRm removes a file or directory. The root is never removable, even with
// recursive: policy lives in the caller, this is the last line of defence.
func afcRm(fs afcFS, remotePath string, recursive bool) error {
	if strings.TrimSpace(remotePath) == "" {
		return errors.New("refusing to remove an empty path")
	}
	remote, err := cleanAfcPath(remotePath)
	if err != nil {
		return err
	}
	if remote == "/" {
		return errors.New("refusing to remove the root directory")
	}
	if recursive {
		return fs.RemoveAll(remote)
	}
	return fs.Remove(remote)
}
