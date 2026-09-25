package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mobile-next/mobilecli/commands"
)

const (
	// screenRecordReadyTimeout bounds how long a start waits for the recording
	// to be confirmed live before giving up. Sized generously above a
	// worst-case cold DeviceKit start on real iOS devices (WDA/app launch +
	// two 10s broadcast-picker button polls + the 5s post-click TCP wait).
	screenRecordReadyTimeout = 60 * time.Second

	// recordingFinalizeTimeout bounds how long a stop waits for its
	// recordings to finish writing their files.
	recordingFinalizeTimeout = 30 * time.Second

	// shutdownFinalizeTimeout bounds how long daemon shutdown waits for
	// in-progress recordings to finish writing.
	shutdownFinalizeTimeout = 10 * time.Second

	// recordingStatus is the status a successful start reports
	recordingStatus = "recording"
)

// RecordingSession holds state for an in-progress screen recording
type RecordingSession struct {
	ID        string
	DeviceID  string
	Output    string
	StartedAt time.Time
	StopChan  chan struct{}
	Ready     chan error // signaled once: nil once recording is confirmed live, or an error if it failed to start
	stopped   bool       // true after StopChan has been closed

	// finished is closed once result is set, so every waiter (stop, a
	// retried stop, shutdown, a timed-out start) reads the same result
	finished chan struct{}
	result   *commands.CommandResponse
}

// finish stores the recorder's result and wakes every waiter. Called once.
func (s *RecordingSession) finish(resp *commands.CommandResponse) {
	s.result = resp
	close(s.finished)
}

// RecordingResult is one stopped recording's result: the recorder's
// response plus the id it was started with.
type RecordingResult struct {
	commands.ScreenRecordResponse
	RecordingID string `json:"recordingId"`
	Error       string `json:"error,omitempty"`
}

// ScreenRecordStartResult answers a started recording.
type ScreenRecordStartResult struct {
	Status      string `json:"status"`
	Output      string `json:"output"`
	RecordingID string `json:"recordingId"`
}

// StopRecordingsResult answers a stop without a recordingId. With exactly one
// recording stopped, its fields are also inlined at the top level, so the
// shape matches a single-recording stop.
type StopRecordingsResult struct {
	*RecordingResult
	Recordings []RecordingResult `json:"recordings"`
}

type recordFunc func(commands.ScreenRecordRequest) *commands.CommandResponse

type resolveDeviceFunc func(deviceID string) (string, error)

// recordingManager tracks every in-progress recording by id. Several may run
// on one device; recorders that cannot share a device refuse a second one
// themselves (see commands.ScreenRecordCommand).
type recordingManager struct {
	mu            sync.Mutex
	sessions      map[string]*RecordingSession
	record        recordFunc
	resolveDevice resolveDeviceFunc
}

var recorder = newRecordingManager(commands.ScreenRecordCommand, resolveRecordingDevice)

func newRecordingManager(record recordFunc, resolve resolveDeviceFunc) *recordingManager {
	return &recordingManager{
		sessions:      map[string]*RecordingSession{},
		record:        record,
		resolveDevice: resolve,
	}
}

// resolveRecordingDevice turns an optional deviceId into the device's id, so
// recordings started without one can still be stopped by device.
func resolveRecordingDevice(deviceID string) (string, error) {
	device, err := commands.FindDeviceOrAutoSelect(deviceID)
	if err != nil {
		return "", err
	}
	return device.ID(), nil
}

// register adds a new session under id, refusing an id already in use.
func (rm *recordingManager) register(id, deviceID, output string) (*RecordingSession, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.sessions[id]; exists {
		return nil, fmt.Errorf("recording %q is already in progress", id)
	}

	s := &RecordingSession{
		ID:        id,
		DeviceID:  deviceID,
		Output:    output,
		StartedAt: time.Now(),
		StopChan:  make(chan struct{}),
		Ready:     make(chan error, 1),
		finished:  make(chan struct{}),
	}
	rm.sessions[id] = s
	return s, nil
}

// remove forgets s, unless its id already belongs to a newer recording.
func (rm *recordingManager) remove(s *RecordingSession) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.sessions[s.ID] == s {
		delete(rm.sessions, s.ID)
	}
}

func (rm *recordingManager) active() bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return len(rm.sessions) > 0
}

// signalStopLocked closes the session's StopChan once. Caller holds rm.mu.
func signalStopLocked(s *RecordingSession) {
	if s.stopped {
		return
	}
	close(s.StopChan)
	s.stopped = true
}

// stopByID signals one recording to stop. A non-empty deviceID must match the
// device the recording runs on.
func (rm *recordingManager) stopByID(id, deviceID string) (*RecordingSession, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	s, ok := rm.sessions[id]
	if !ok || (deviceID != "" && s.DeviceID != deviceID) {
		return nil, fmt.Errorf("no recording in progress with id %q", id)
	}

	signalStopLocked(s)
	return s, nil
}

// stopMatching signals every recording on deviceID to stop, or every
// recording when deviceID is empty. The sessions stay registered until their
// results are read.
func (rm *recordingManager) stopMatching(deviceID string) ([]*RecordingSession, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	var stopped []*RecordingSession
	for _, s := range rm.sessions {
		if deviceID != "" && s.DeviceID != deviceID {
			continue
		}
		signalStopLocked(s)
		stopped = append(stopped, s)
	}

	if len(stopped) == 0 {
		return nil, fmt.Errorf("no recording in progress")
	}
	return stopped, nil
}

// start begins a recording and returns once it is confirmed live.
func (rm *recordingManager) start(p ScreenRecordParams) (ScreenRecordStartResult, error) {
	if p.Output == "" {
		return ScreenRecordStartResult{}, fmt.Errorf("'output' is required")
	}

	id := p.RecordingID
	if id == "" {
		id = uuid.NewString()
	}

	deviceID, err := rm.resolveDevice(p.DeviceID)
	if err != nil {
		return ScreenRecordStartResult{}, fmt.Errorf("failed to start recording: %w", err)
	}

	session, err := rm.register(id, deviceID, p.Output)
	if err != nil {
		return ScreenRecordStartResult{}, err
	}

	req := commands.ScreenRecordRequest{
		DeviceID:   deviceID,
		OutputPath: p.Output,
		TimeLimit:  p.TimeLimit,
		StopChan:   session.StopChan,
		Ready:      session.Ready,
		Silent:     true,
	}

	go func() {
		session.finish(rm.record(req))
	}()

	if err := rm.awaitStart(session); err != nil {
		return ScreenRecordStartResult{}, err
	}

	return ScreenRecordStartResult{Status: recordingStatus, Output: p.Output, RecordingID: id}, nil
}

// awaitStart doesn't ack until the recording is actually confirmed live. On
// real iOS devices this waits for the ReplayKit broadcast picker to be
// clicked, so callers never race a still-starting recording with device
// commands.
func (rm *recordingManager) awaitStart(session *RecordingSession) error {
	select {
	case readyErr := <-session.Ready:
		if readyErr != nil {
			rm.remove(session)
			return fmt.Errorf("failed to start recording: %w", readyErr)
		}
		return nil
	case <-session.finished:
		resp := session.result
		// recording finished (or failed) before ever confirming it was live
		rm.remove(session)
		if resp.Status == statusError {
			return fmt.Errorf("%s", resp.Error)
		}
		return fmt.Errorf("recording ended before it was confirmed started")
	case <-time.After(screenRecordReadyTimeout):
		// the command goroutine may still be starting (or even recording);
		// ask it to stop and wait for it to exit before freeing the session,
		// so it cannot overlap a subsequent capture
		if _, stopErr := rm.stopByID(session.ID, ""); stopErr == nil {
			select {
			case <-session.finished:
			case <-time.After(recordingFinalizeTimeout):
			}
		}

		rm.remove(session)
		return fmt.Errorf("timed out waiting for recording to start")
	}
}

// stopOne stops the recording with id and returns its result. A non-empty
// deviceID must match the device the recording runs on.
func (rm *recordingManager) stopOne(id, deviceID string) (RecordingResult, error) {
	session, err := rm.stopByID(id, deviceID)
	if err != nil {
		return RecordingResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), recordingFinalizeTimeout)
	defer cancel()
	return rm.awaitResult(ctx, session)
}

// stopAll stops every recording on deviceID, or every recording at all when
// deviceID is empty.
func (rm *recordingManager) stopAll(deviceID string) (StopRecordingsResult, error) {
	sessions, err := rm.stopMatching(deviceID)
	if err != nil {
		return StopRecordingsResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), recordingFinalizeTimeout)
	defer cancel()

	if len(sessions) > 1 {
		return StopRecordingsResult{Recordings: rm.awaitResults(ctx, sessions)}, nil
	}

	// a lone recording keeps the single-recording shape, errors included
	result, err := rm.awaitResult(ctx, sessions[0])
	if err != nil {
		return StopRecordingsResult{}, err
	}
	return StopRecordingsResult{RecordingResult: &result, Recordings: []RecordingResult{result}}, nil
}

// awaitResults collects every stopped session's result; a failed recording
// reports its error in its own entry instead of failing the others.
func (rm *recordingManager) awaitResults(ctx context.Context, sessions []*RecordingSession) []RecordingResult {
	results := make([]RecordingResult, 0, len(sessions))
	for _, s := range sessions {
		result, err := rm.awaitResult(ctx, s)
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

// awaitResult waits for a stopped session to finish writing, then forgets it.
func (rm *recordingManager) awaitResult(ctx context.Context, s *RecordingSession) (RecordingResult, error) {
	defer rm.remove(s)

	result := RecordingResult{RecordingID: s.ID}
	select {
	case <-s.finished:
		resp := s.result
		if resp.Status == statusError {
			return result, fmt.Errorf("%s", resp.Error)
		}
		result.ScreenRecordResponse = enrichWithDuration(resp.Data, s.StartedAt)
		return result, nil
	case <-ctx.Done():
		return result, fmt.Errorf("timeout waiting for recording to finalize")
	}
}

func enrichWithDuration(data any, startedAt time.Time) commands.ScreenRecordResponse {
	m, _ := data.(commands.ScreenRecordResponse)
	if m.Duration == "" {
		m.Duration = time.Since(startedAt).Round(time.Millisecond).String()
	}
	return m
}

// stopAllForShutdown stops every recording and waits briefly for each to
// finish writing.
func (rm *recordingManager) stopAllForShutdown() {
	sessions, err := rm.stopMatching("")
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownFinalizeTimeout)
	defer cancel()
	rm.awaitResults(ctx, sessions)
}

// StopRecordingForShutdown stops every in-progress recording and waits
// briefly for them to finish writing, so the daemon never exits mid-file.
func StopRecordingForShutdown() {
	recorder.stopAllForShutdown()
}
