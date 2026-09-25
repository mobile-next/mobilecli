package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fakeDeviceID      = "fake-device"
	fakeFrameInterval = 5 * time.Millisecond
	waitForFrames     = 2 * time.Second
)

// fakeSharedStream stands in for a device's shared H.264 stream: it emits a
// frame every fakeFrameInterval to every current subscriber.
type fakeSharedStream struct {
	mu          sync.Mutex
	subscribers map[chan []byte]struct{}
	done        chan struct{}
}

func startFakeSharedStream(t *testing.T) *fakeSharedStream {
	t.Helper()
	s := &fakeSharedStream{subscribers: map[chan []byte]struct{}{}, done: make(chan struct{})}
	go s.emitFrames()
	t.Cleanup(func() { close(s.done) })
	return s
}

func (s *fakeSharedStream) emitFrames() {
	ticker := time.NewTicker(fakeFrameInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.broadcast([]byte("frame\n"))
		}
	}
}

func (s *fakeSharedStream) broadcast(frame []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- frame:
		default:
		}
	}
}

func (s *fakeSharedStream) subscribe() chan []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan []byte, 64)
	s.subscribers[ch] = struct{}{}
	return ch
}

func (s *fakeSharedStream) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, ch)
}

// record is a recorder that subscribes to the shared stream and writes every
// frame to its own output file until stopped, like the real avc path.
func (s *fakeSharedStream) record(req commands.ScreenRecordRequest) *commands.CommandResponse {
	frames := s.subscribe()
	defer s.unsubscribe(frames)

	out, err := os.Create(req.OutputPath)
	if err != nil {
		req.Ready <- err
		return commands.NewErrorResponse(err)
	}
	defer func() { _ = out.Close() }()

	req.Ready <- nil
	count := 0
	for {
		select {
		case <-req.StopChan:
			return commands.NewSuccessResponse(commands.ScreenRecordResponse{Output: req.OutputPath, FrameCount: count})
		case frame := <-frames:
			if _, err := out.Write(frame); err != nil {
				return commands.NewErrorResponse(err)
			}
			count++
		}
	}
}

// useRecorderOnFakeStream swaps the server's recorder for one that records
// the fake shared stream, for the duration of the test. It returns the
// directory for the test's output files, created before the stop cleanup is
// registered so the recordings stop before the directory is removed.
func useRecorderOnFakeStream(t *testing.T) (outputDir string) {
	t.Helper()
	outputDir = t.TempDir()
	stream := startFakeSharedStream(t)
	previous := recorder
	recorder = newRecordingManager(stream.record, func(string) (string, error) { return fakeDeviceID, nil })
	t.Cleanup(func() {
		recorder.stopAllForShutdown()
		recorder = previous
	})
	return outputDir
}

func rpcParams(t *testing.T, fields map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(fields)
	require.NoError(t, err)
	return raw
}

func startRecording(t *testing.T, recordingID, output string) (ScreenRecordStartResult, error) {
	t.Helper()
	result, err := handleScreenRecord(rpcParams(t, map[string]any{
		"deviceId":    fakeDeviceID,
		"output":      output,
		"recordingId": recordingID,
	}))
	if err != nil {
		return ScreenRecordStartResult{}, err
	}
	started, ok := result.(ScreenRecordStartResult)
	require.True(t, ok, "start returned %T", result)
	return started, nil
}

func mustStartRecording(t *testing.T, recordingID, output string) ScreenRecordStartResult {
	t.Helper()
	result, err := startRecording(t, recordingID, output)
	require.NoError(t, err)
	return result
}

func stopRecording(t *testing.T, fields map[string]any) (any, error) {
	t.Helper()
	return handleScreenRecordStop(rpcParams(t, fields))
}

func asStopAllResult(t *testing.T, result any) StopRecordingsResult {
	t.Helper()
	stopped, ok := result.(StopRecordingsResult)
	require.True(t, ok, "stop returned %T", result)
	return stopped
}

func asStopOneResult(t *testing.T, result any) RecordingResult {
	t.Helper()
	stopped, ok := result.(RecordingResult)
	require.True(t, ok, "stop returned %T", result)
	return stopped
}

func outputPath(dir, name string) string {
	return filepath.Join(dir, name)
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func waitUntilFileHasData(t *testing.T, path string) {
	t.Helper()
	require.Eventually(t, func() bool { return fileSize(path) > 0 }, waitForFrames, fakeFrameInterval, "%s never received frames", path)
}

func waitUntilFileGrowsPast(t *testing.T, path string, size int64) {
	t.Helper()
	require.Eventually(t, func() bool { return fileSize(path) > size }, waitForFrames, fakeFrameInterval, "%s stopped growing", path)
}

func jsonKeys(t *testing.T, value any) []string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return keys
}

func TestTwoConcurrentRecordingsOnASharedStreamBothProduceFiles(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	first, second := outputPath(dir, "first.mp4"), outputPath(dir, "second.mp4")

	mustStartRecording(t, "first", first)
	mustStartRecording(t, "second", second)
	waitUntilFileHasData(t, first)
	waitUntilFileHasData(t, second)

	result, err := stopRecording(t, map[string]any{"deviceId": fakeDeviceID})
	require.NoError(t, err)

	stopped := asStopAllResult(t, result)
	assert.Len(t, stopped.Recordings, 2)
	for _, recording := range stopped.Recordings {
		assert.Positive(t, recording.FrameCount, "recording %s", recording.RecordingID)
		assert.Empty(t, recording.Error)
	}
}

func TestStoppingOneRecordingByIDLeavesTheOtherRunning(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	first, second := outputPath(dir, "first.mp4"), outputPath(dir, "second.mp4")
	mustStartRecording(t, "first", first)
	mustStartRecording(t, "second", second)

	result, err := stopRecording(t, map[string]any{"recordingId": "first"})
	require.NoError(t, err)

	stopped := asStopOneResult(t, result)
	assert.Equal(t, "first", stopped.RecordingID)
	assert.Equal(t, first, stopped.Output)
	waitUntilFileGrowsPast(t, second, fileSize(second))
	assert.True(t, recorder.active(), "the second recording must still be running")
}

func TestStopWithoutAnIDStopsEveryRecording(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	mustStartRecording(t, "first", outputPath(dir, "first.mp4"))
	mustStartRecording(t, "second", outputPath(dir, "second.mp4"))

	result, err := stopRecording(t, map[string]any{})
	require.NoError(t, err)

	assert.Len(t, asStopAllResult(t, result).Recordings, 2)
	assert.False(t, recorder.active())
}

func TestStartWithARecordingIDAlreadyRunningFails(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	mustStartRecording(t, "same", outputPath(dir, "first.mp4"))

	_, err := startRecording(t, "same", outputPath(dir, "second.mp4"))

	assert.ErrorContains(t, err, "already in progress")
}

func TestStopWithAnUnknownRecordingIDFails(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	mustStartRecording(t, "known", outputPath(dir, "first.mp4"))

	_, err := stopRecording(t, map[string]any{"recordingId": "unknown"})

	assert.ErrorContains(t, err, "unknown")
	assert.True(t, recorder.active(), "a failed stop must not touch the running recording")
}

func TestStopWhenNothingIsRecordingReportsNoRecording(t *testing.T) {
	useRecorderOnFakeStream(t)

	_, err := stopRecording(t, map[string]any{})

	assert.EqualError(t, err, "no recording in progress")
}

func TestStartWithoutARecordingIDGeneratesOne(t *testing.T) {
	dir := useRecorderOnFakeStream(t)

	started := mustStartRecording(t, "", outputPath(dir, "first.mp4"))

	assert.Len(t, started.RecordingID, len("00000000-0000-0000-0000-000000000000"))
	assert.Equal(t, "recording", started.Status)
}

func TestStopWithoutAnIDKeepsTheSingleRecordingResponseShape(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	output := outputPath(dir, "only.mp4")
	started := mustStartRecording(t, "", output)

	result, err := stopRecording(t, map[string]any{"deviceId": fakeDeviceID})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"output", "frameCount", "duration", "recordingId", "recordings"}, jsonKeys(t, result))
	stopped := asStopAllResult(t, result)
	assert.Equal(t, output, stopped.Output)
	assert.Equal(t, started.RecordingID, stopped.RecordingID)
	assert.Len(t, stopped.Recordings, 1)
}

func TestStopOnAnotherDeviceLeavesThisDevicesRecordings(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	mustStartRecording(t, "first", outputPath(dir, "first.mp4"))

	_, err := stopRecording(t, map[string]any{"deviceId": "other-device"})

	assert.EqualError(t, err, "no recording in progress")
	assert.True(t, recorder.active())
}

func TestShutdownStopsEveryRecording(t *testing.T) {
	dir := useRecorderOnFakeStream(t)
	mustStartRecording(t, "first", outputPath(dir, "first.mp4"))
	mustStartRecording(t, "second", outputPath(dir, "second.mp4"))

	StopRecordingForShutdown()

	assert.False(t, recorder.active())
}
