package devices

import (
	"fmt"
	"path"
	"strings"
)

// cleanContainerRelativePath cleans a path that is relative to an app's data
// container and returns it rooted ("/files/x", or "/" for the container
// itself). A path whose ".." would climb above the container is rejected
// rather than clamped, so `fs rm -r <bundle> /../..` fails instead of quietly
// meaning the whole container.
func cleanContainerRelativePath(remotePath string) (string, error) {
	rel := path.Clean(strings.TrimPrefix(remotePath, "/"))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("path '%s' leaves the app container", remotePath)
	}
	if rel == "." {
		return "/", nil
	}
	return "/" + rel, nil
}
