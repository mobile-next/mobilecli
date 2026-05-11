package devices

import "errors"

func (r *RemoteDevice) PushFile(bundleID, localPath, remotePath string) error {
	return errors.New("not implemented")
}

func (r *RemoteDevice) PullFile(bundleID, remotePath, localPath string) error {
	return errors.New("not implemented")
}

func (r *RemoteDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	return nil, errors.New("not implemented")
}

func (r *RemoteDevice) Mkdir(bundleID, remotePath string, parents bool) error {
	return errors.New("not implemented")
}

func (r *RemoteDevice) Rm(bundleID, remotePath string, recursive bool) error {
	return errors.New("not implemented")
}

func (r *RemoteDevice) GetAppContainerPath(bundleID string) (string, error) {
	return "", errors.New("not implemented")
}
