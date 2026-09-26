package server

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/mobile-next/mobilecli/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const quickly = time.Second

// finishingRecorder confirms each recording live, then answers response
// once stopped and release is closed, as the recording goroutine does after
// finalizing its file.
type finishingRecorder struct {
	response commands.ScreenRecordResponse
	release  chan struct{}
}

func (f finishingRecorder) record(req commands.ScreenRecordRequest) *commands.CommandResponse {
	req.Ready <- nil
	<-req.StopChan
	<-f.release
	response := f.response
	response.Output = req.OutputPath
	return commands.NewSuccessResponse(response)
}

// useRecorderThatFinishesWith swaps the server's recorder for one whose
// recordings end with response. Finalizing waits for the returned release
// func, unless the test never calls it.
func useRecorderThatFinishesWith(t *testing.T, response commands.ScreenRecordResponse) (release func()) {
	t.Helper()
	fake := finishingRecorder{response: response, release: make(chan struct{})}
	var once sync.Once
	release = func() { once.Do(func() { close(fake.release) }) }

	previous := recorder
	recorder = newRecordingManager(fake.record, func(string) (string, error) { return fakeDeviceID, nil })
	t.Cleanup(func() {
		release()
		recorder.stopAllForShutdown()
		recorder = previous
	})
	return release
}

func useRecorderThatFinishesAtOnce(t *testing.T, response commands.ScreenRecordResponse) {
	t.Helper()
	useRecorderThatFinishesWith(t, response)()
}

func stopEveryRecording(t *testing.T) StopRecordingsResult {
	t.Helper()
	result, err := handleScreenRecordStop(nil)
	require.NoError(t, err)
	stopped, ok := result.(StopRecordingsResult)
	require.True(t, ok, "stop returned %T", result)
	return stopped
}

func TestANewRecordingCanStartRightAfterTheLastOneStopped(t *testing.T) {
	useRecorderThatFinishesAtOnce(t, commands.ScreenRecordResponse{})
	mustStartRecording(t, "", "/tmp/first.mp4")
	stopEveryRecording(t)

	_, err := startRecording(t, "", "/tmp/second.mp4")

	assert.NoError(t, err, "a recording started right after stop was refused")
}

func TestStopReportsARecordingEndedByTheSizeLimit(t *testing.T) {
	useRecorderThatFinishesAtOnce(t, commands.ScreenRecordResponse{EndReason: "size_limit"})
	mustStartRecording(t, "", "/tmp/full.mp4")

	stopped := stopEveryRecording(t)

	assert.Equal(t, "size_limit", stopped.EndReason)
	raw, err := json.Marshal(stopped)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"endReason":"size_limit"`)
}

func TestStopByIDReportsARecordingEndedByTheSizeLimit(t *testing.T) {
	useRecorderThatFinishesAtOnce(t, commands.ScreenRecordResponse{EndReason: "size_limit"})
	mustStartRecording(t, "full", "/tmp/full.mp4")

	result, err := stopRecording(t, map[string]any{"recordingId": "full"})

	require.NoError(t, err)
	assert.Equal(t, "size_limit", asStopOneResult(t, result).EndReason)
}

// awaitBothStops waits for two stops of the same recording at once and
// returns how each one ended.
func awaitBothStops(first, second *RecordingSession) []error {
	ctx, cancel := context.WithTimeout(context.Background(), quickly)
	defer cancel()

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, session := range []*RecordingSession{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = recorder.awaitResult(ctx, session)
		}()
	}
	wg.Wait()
	return errs
}

func TestTwoStopsOfTheSameRecordingBothGetItsResult(t *testing.T) {
	release := useRecorderThatFinishesWith(t, commands.ScreenRecordResponse{})
	mustStartRecording(t, "twice", "/tmp/twice.mp4")
	first, err := recorder.stopByID("twice", "")
	require.NoError(t, err)
	retried, err := recorder.stopByID("twice", "")
	require.NoError(t, err)

	release()
	errs := awaitBothStops(first, retried)

	assert.NoError(t, errs[0], "first stop")
	assert.NoError(t, errs[1], "retried stop")
}

func TestShutdownDuringAStopGetsTheResultToo(t *testing.T) {
	release := useRecorderThatFinishesWith(t, commands.ScreenRecordResponse{})
	mustStartRecording(t, "overlap", "/tmp/overlap.mp4")
	stopping, err := recorder.stopByID("overlap", "")
	require.NoError(t, err)
	shuttingDown, err := recorder.stopMatching("")
	require.NoError(t, err)

	release()
	errs := awaitBothStops(stopping, shuttingDown[0])

	assert.NoError(t, errs[0], "stop")
	assert.NoError(t, errs[1], "shutdown")
}

func TestAStopThatTimesOutKeepsTheRecordingUntilItFinishes(t *testing.T) {
	release := useRecorderThatFinishesWith(t, commands.ScreenRecordResponse{})
	mustStartRecording(t, "slow", "/tmp/slow.mp4")
	session, err := recorder.stopByID("slow", "")
	require.NoError(t, err)
	expired, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = recorder.awaitResult(expired, session)
	require.ErrorContains(t, err, "timeout")
	_, err = startRecording(t, "slow", "/tmp/slow-again.mp4")
	assert.ErrorContains(t, err, "already in progress", "the id was freed while its recorder still runs")

	release()
	assert.Eventually(t, func() bool { return !recorder.active() }, quickly, 10*time.Millisecond)
}

// endsOnItsOwn is a recorder that stops by itself right after going live, as
// the size safety stop or a time limit ends a recording.
func endsOnItsOwn(req commands.ScreenRecordRequest) *commands.CommandResponse {
	req.Ready <- nil
	return commands.NewSuccessResponse(commands.ScreenRecordResponse{Output: req.OutputPath, EndReason: "size_limit"})
}

func TestARecordingThatEndedOnItsOwnWaitsForAStopToReadIt(t *testing.T) {
	previous := recorder
	recorder = newRecordingManager(endsOnItsOwn, func(string) (string, error) { return fakeDeviceID, nil })
	t.Cleanup(func() { recorder = previous })
	mustStartRecording(t, "self-ended", "/tmp/self-ended.mp4")
	require.Eventually(t, func() bool { return recordingHasExited(t, "self-ended") }, quickly, 10*time.Millisecond)

	result, err := recorder.stopOne("self-ended", "")

	require.NoError(t, err)
	assert.Equal(t, "size_limit", result.EndReason)
	assert.False(t, recorder.active(), "a read recording is forgotten")
}

func recordingHasExited(t *testing.T, id string) bool {
	t.Helper()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	s, ok := recorder.sessions[id]
	return ok && s.exited
}
