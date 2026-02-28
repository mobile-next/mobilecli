package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/mobile-next/mobilecli/commands"
)

// RecordingSession holds state for an in-progress screen recording
type RecordingSession struct {
	Output    string
	StartedAt time.Time
	StopChan  chan struct{}
	Done      chan *commands.CommandResponse
	stopped   bool // true after StopChan has been closed
}

type recordingManager struct {
	mu      sync.Mutex
	session *RecordingSession
}

var recorder = &recordingManager{}

func (rm *recordingManager) start(output string) (*RecordingSession, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.session != nil {
		return nil, fmt.Errorf("a recording is already in progress")
	}

	s := &RecordingSession{
		Output:    output,
		StartedAt: time.Now(),
		StopChan:  make(chan struct{}),
		Done:      make(chan *commands.CommandResponse, 1),
	}
	rm.session = s
	return s, nil
}

// stop returns the current session and closes its StopChan (idempotent).
// the session is not cleared here — the caller reads from Done, then calls clear.
func (rm *recordingManager) stop() (*RecordingSession, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.session == nil {
		return nil, fmt.Errorf("no recording in progress")
	}

	s := rm.session
	if !s.stopped {
		close(s.StopChan)
		s.stopped = true
	}
	return s, nil
}

func (rm *recordingManager) clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.session = nil
}
