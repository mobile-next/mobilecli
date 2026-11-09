package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// HomeEnv overrides the directory holding the daemon's socket, pid and log files.
const HomeEnv = "MOBILECLI_HOME"

// FilePaths are the on-disk files a daemon owns.
type FilePaths struct {
	Dir    string
	Socket string
	Pid    string
	Log    string
}

// Dir returns $MOBILECLI_HOME, or ~/.mobilecli.
func Dir() (string, error) {
	if dir := os.Getenv(HomeEnv); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	return filepath.Join(home, ".mobilecli"), nil
}

// maxSocketPathLen is sizeof(sun_path) minus the NUL terminator.
func maxSocketPathLen() int {
	if runtime.GOOS == "darwin" {
		return 103
	}
	return 107
}

// Paths resolves all daemon file paths and validates the socket path length.
func Paths() (FilePaths, error) {
	dir, err := Dir()
	if err != nil {
		return FilePaths{}, err
	}

	p := FilePaths{
		Dir:    dir,
		Socket: filepath.Join(dir, "daemon.sock"),
		Pid:    filepath.Join(dir, "daemon.pid"),
		Log:    filepath.Join(dir, "daemon.log"),
	}

	if len(p.Socket) > maxSocketPathLen() {
		return FilePaths{}, fmt.Errorf("socket path %q is too long (%d bytes, max %d); set %s to a shorter directory", p.Socket, len(p.Socket), maxSocketPathLen(), HomeEnv)
	}

	return p, nil
}
