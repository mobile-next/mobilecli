package devices

import "errors"

func (d IOSDevice) PushFile(bundleID, localPath, remotePath string) error {
	return errors.New("not implemented")
}

func (d IOSDevice) PullFile(bundleID, remotePath, localPath string) error {
	return errors.New("not implemented")
}

func (d IOSDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	return nil, errors.New("not implemented")
}

func (d IOSDevice) Mkdir(bundleID, remotePath string) error {
	return errors.New("not implemented")
}

func (d IOSDevice) Rm(bundleID, remotePath string) error {
	return errors.New("not implemented")
}

func (d IOSDevice) GetAppContainerPath(bundleID string) (string, error) {
	return "", errors.New("not implemented")
}
