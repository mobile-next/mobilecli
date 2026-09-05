package daemon

import (
	"encoding/json"
	"os"
	"time"
)

type pidInfo struct {
	PID       int       `json:"pid"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"startedAt"`
}

func writePidFile(path string, info pidInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readPidFile(path string) (pidInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pidInfo{}, err
	}
	var info pidInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return pidInfo{}, err
	}
	return info, nil
}
