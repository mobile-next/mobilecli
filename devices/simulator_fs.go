package devices

import "errors"

func (s *SimulatorDevice) PushFile(bundleID, localPath, remotePath string) error {
	return errors.New("not implemented")
}

func (s *SimulatorDevice) PullFile(bundleID, remotePath, localPath string) error {
	return errors.New("not implemented")
}

func (s *SimulatorDevice) ListFiles(bundleID, remotePath string) ([]FileEntry, error) {
	return nil, errors.New("not implemented")
}

func (s *SimulatorDevice) Mkdir(bundleID, remotePath string) error {
	return errors.New("not implemented")
}

func (s *SimulatorDevice) Rm(bundleID, remotePath string) error {
	return errors.New("not implemented")
}

func (s *SimulatorDevice) GetAppContainerPath(bundleID string) (string, error) {
	return "", errors.New("not implemented")
}
