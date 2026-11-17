package commands

import (
	"path/filepath"
	"sync"
)

// recordingPaths holds the generated output names of recordings in progress.
// Such a file is only written when its recording ends, so until then nothing
// on disk shows the name is taken.
var recordingPaths = struct {
	mu       sync.Mutex
	reserved map[string]bool
}{reserved: map[string]bool{}}

// reserveRecordingPath picks the first name from path, path-1, path-2, ...
// that is neither on disk nor reserved by a recording in progress, and
// reserves it. The returned release must be called once the recording has
// ended.
func reserveRecordingPath(path string) (string, func()) {
	recordingPaths.mu.Lock()
	defer recordingPaths.mu.Unlock()

	// a directory reached through a symlink is the same directory: reserve
	// under its resolved spelling, return the path as the caller spelled it
	dir := filepath.Dir(path)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	reservationKey := func(candidate string) string {
		return filepath.Join(dir, filepath.Base(candidate))
	}

	free := firstFreePath(path, func(candidate string) bool {
		return recordingPaths.reserved[reservationKey(candidate)] || fileExists(candidate)
	})
	key := reservationKey(free)
	recordingPaths.reserved[key] = true
	release := func() {
		recordingPaths.mu.Lock()
		defer recordingPaths.mu.Unlock()
		delete(recordingPaths.reserved, key)
	}
	return free, release
}
