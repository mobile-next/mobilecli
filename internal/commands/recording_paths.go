package commands

import "sync"

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

	free := firstFreePath(path, func(candidate string) bool {
		return recordingPaths.reserved[candidate] || fileExists(candidate)
	})
	recordingPaths.reserved[free] = true
	release := func() {
		recordingPaths.mu.Lock()
		defer recordingPaths.mu.Unlock()
		delete(recordingPaths.reserved, free)
	}
	return free, release
}
