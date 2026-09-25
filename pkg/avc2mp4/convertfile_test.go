package avc2mp4

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// parameter sets the muxer can read a resolution from
var (
	realSPS = []byte{0x67, 0x64, 0x00, 0x0A, 0xAC, 0x72, 0x84, 0x44, 0x26, 0x84, 0x00, 0x00,
		0x03, 0x00, 0x04, 0x00, 0x00, 0x03, 0x00, 0xCA, 0x3C, 0x48, 0x96, 0x11, 0x80}
	realPPS = []byte{0x68, 0xE8, 0x43, 0x8F, 0x13, 0x21, 0x30}
)

// 0x88 / 0x9A start the slice header with first_mb_in_slice = 0, so each slice
// opens a new picture
func keyFrame(marker byte) []byte   { return []byte{0x65, 0x88, marker} }
func deltaFrame(marker byte) []byte { return []byte{0x41, 0x9A, marker} }

// recordedStream is what a recording spools to disk: one GOP after another,
// every picture stamped by the timecode SEI in front of it
func recordedStream(pictures int) []byte {
	var units [][]byte
	for i := range pictures {
		if i%30 == 0 {
			units = append(units, realSPS, realPPS)
		}
		units = append(units, buildSEINalu(mobilenxTimecodeUUID, uint64(5_000_000+i*16_666)))
		if i%30 == 0 {
			units = append(units, keyFrame(byte(i)))
		} else {
			units = append(units, deltaFrame(byte(i)))
		}
	}
	return annexB(startCode4, units...)
}

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recording.avc")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing input: %v", err)
	}
	return path
}

func convertInMemory(t *testing.T, data []byte) ([]byte, *ConvertResult) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in-memory.mp4")
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating output: %v", err)
	}
	result, err := Convert(data, out)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("Convert failed: %v", err)
	}
	mp4, _ := os.ReadFile(path)
	return mp4, result
}

func convertFromDisk(t *testing.T, data []byte) ([]byte, *ConvertResult, error) {
	t.Helper()
	in, err := os.Open(writeTempFile(t, data))
	if err != nil {
		t.Fatalf("opening input: %v", err)
	}
	defer func() { _ = in.Close() }()

	path := filepath.Join(t.TempDir(), "from-disk.mp4")
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating output: %v", err)
	}
	result, err := ConvertFile(in, out)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	mp4, _ := os.ReadFile(path)
	return mp4, result, err
}

func TestConvertFileWritesTheSameMp4AsConvert(t *testing.T) {
	stream := recordedStream(95)

	want, wantResult := convertInMemory(t, stream)
	got, gotResult, err := convertFromDisk(t, stream)
	if err != nil {
		t.Fatalf("ConvertFile failed: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("mp4 differs: %d bytes from disk, %d bytes in memory", len(got), len(want))
	}
	if *gotResult != *wantResult {
		t.Fatalf("result differs: %+v from disk, %+v in memory", gotResult, wantResult)
	}
}

func TestConvertFileReportsEveryPictureAndTheSpanBetweenThem(t *testing.T) {
	_, result, err := convertFromDisk(t, recordedStream(61))
	if err != nil {
		t.Fatalf("ConvertFile failed: %v", err)
	}

	if result.FrameCount != 61 {
		t.Errorf("expected 61 frames, got %d", result.FrameCount)
	}
	if want := 60 * 16_666_000; result.Duration.Nanoseconds() != int64(want) {
		t.Errorf("expected duration %dns, got %s", want, result.Duration)
	}
}

func TestConvertFilePutsTheMovieHeaderAfterTheSamples(t *testing.T) {
	mp4, _, err := convertFromDisk(t, recordedStream(31))
	if err != nil {
		t.Fatalf("ConvertFile failed: %v", err)
	}

	mdat, moov := bytes.Index(mp4, []byte("mdat")), bytes.Index(mp4, []byte("moov"))
	if mdat < 0 || moov < 0 || moov < mdat {
		t.Fatalf("expected mdat before moov, got mdat at %d, moov at %d", mdat, moov)
	}
}

func TestConvertFileReportsNoFramesForAStreamWithoutTimestamps(t *testing.T) {
	stream := annexB(startCode4, realSPS, realPPS, keyFrame(1))

	_, _, err := convertFromDisk(t, stream)
	if !errors.Is(err, ErrNoFrames) {
		t.Fatalf("expected ErrNoFrames, got %v", err)
	}
}

func TestConvertFileMuxesAStreamCutOffMidPicture(t *testing.T) {
	// a recording cut off mid-write still holds every earlier picture
	stream := recordedStream(10)
	cut := stream[:len(stream)-1]

	_, result, err := convertFromDisk(t, cut)
	if err != nil {
		t.Fatalf("ConvertFile failed: %v", err)
	}
	if result.FrameCount != 10 {
		t.Fatalf("expected 10 frames (the last one truncated), got %d", result.FrameCount)
	}
}
