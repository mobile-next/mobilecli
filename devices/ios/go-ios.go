package ios

import (
	"fmt"
	"os"
	"os/exec"
)

func FindGoIosPath() (string, error) {
	if envPath := os.Getenv("GO_IOS_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	if path, err := exec.LookPath("go-ios"); err == nil {
		return path, nil
	}

	if path, err := exec.LookPath("ios"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("neither go-ios nor ios found in PATH")
}

