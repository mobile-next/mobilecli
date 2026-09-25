package server

import (
	"testing"

	"github.com/mobile-next/mobilecli/commands"
)

// aRecordingThatFinished registers a recording whose capture already ended
// with response, as the recording goroutine leaves it
func aRecordingThatFinished(t *testing.T, output string, response commands.ScreenRecordResponse) {
	t.Helper()
	session, err := recorder.start(output)
	if err != nil {
		t.Fatalf("starting the recording: %v", err)
	}
	session.Done <- commands.NewSuccessResponse(response)
}

func stopTheRecording(t *testing.T) commands.ScreenRecordResponse {
	t.Helper()
	result, err := handleScreenRecordStop(nil)
	if err != nil {
		t.Fatalf("stopping the recording: %v", err)
	}
	response, ok := result.(commands.ScreenRecordResponse)
	if !ok {
		t.Fatalf("stop returned %T, want ScreenRecordResponse", result)
	}
	return response
}

func TestANewRecordingCanStartRightAfterTheLastOneStopped(t *testing.T) {
	aRecordingThatFinished(t, "/tmp/first.mp4", commands.ScreenRecordResponse{Output: "/tmp/first.mp4"})
	stopTheRecording(t)

	_, err := recorder.start("/tmp/second.mp4")
	if err != nil {
		t.Fatalf("a recording started right after stop was refused: %v", err)
	}
	recorder.clear()
}

func TestStopReportsARecordingEndedByTheSizeLimit(t *testing.T) {
	aRecordingThatFinished(t, "/tmp/full.mp4", commands.ScreenRecordResponse{Output: "/tmp/full.mp4", EndReason: "size_limit"})

	if response := stopTheRecording(t); response.EndReason != "size_limit" {
		t.Fatalf("expected endReason size_limit, got %q", response.EndReason)
	}
}
