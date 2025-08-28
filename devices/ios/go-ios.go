package ios

import (
	"fmt"
)

func FindGoIosPath() (string, error) {
	return "", fmt.Errorf("neither go-ios nor ios found in PATH")
}
