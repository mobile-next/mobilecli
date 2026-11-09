package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mobile-next/mobilecli/pkg/avc2mp4"
)

var fourByteStartCode = []byte{0x00, 0x00, 0x00, 0x01}

func withStartCode(payload []byte) []byte {
	return append(bytes.Clone(fourByteStartCode), payload...)
}

// parameter sets the muxer can read a resolution from
var (
	sequenceParameterSet = withStartCode([]byte{0x67, 0x64, 0x00, 0x0A, 0xAC, 0x72, 0x84, 0x44, 0x26, 0x84,
		0x00, 0x00, 0x03, 0x00, 0x04, 0x00, 0x00, 0x03, 0x00, 0xCA, 0x3C, 0x48, 0x96, 0x11, 0x80})
	pictureParameterSet = withStartCode([]byte{0x68, 0xE8, 0x43, 0x8F, 0x13, 0x21, 0x30})
)

// timecode is the SEI the encoder puts in front of every picture
func timecode(timestampUs uint64) []byte {
	payload := []byte{0x06, 0x05, 24}
	payload = append(payload, []byte("MOBILENXTIMECODE")...)
	for shift := 56; shift >= 0; shift -= 8 {
		payload = append(payload, byte(timestampUs>>shift))
	}
	return withStartCode(append(payload, 0x80))
}

func keyFrameSlice(marker byte) []byte   { return withStartCode([]byte{0x65, 0x88, marker, 0xFF}) }
func deltaFrameSlice(marker byte) []byte { return withStartCode([]byte{0x41, 0x9A, marker, 0xFF}) }

// oneSecondOfPictures is a GOP: parameter sets, a key frame, then delta frames,
// each picture stamped with its timecode
func oneSecondOfPictures(firstTimestampUs uint64, pictures int) [][]byte {
	units := [][]byte{sequenceParameterSet, pictureParameterSet}
	for i := range pictures {
		units = append(units, timecode(firstTimestampUs+uint64(i)*16_666))
		if i == 0 {
			units = append(units, keyFrameSlice(byte(i)))
		} else {
			units = append(units, deltaFrameSlice(byte(i)))
		}
	}
	return units
}

// inChunksOf cuts the stream the way the device delivers it: at arbitrary
// offsets, not at unit boundaries
func inChunksOf(size int, stream []byte) [][]byte {
	var chunks [][]byte
	for len(stream) > size {
		chunks = append(chunks, stream[:size])
		stream = stream[size:]
	}
	return append(chunks, stream)
}

func spoolEverything(spool *avcSpool, stream []byte) (acceptedEveryChunk bool) {
	for _, chunk := range inChunksOf(5, stream) {
		if !spool.write(chunk) {
			return false
		}
	}
	return true
}

func muxedFrameCount(t *testing.T, avc []byte) int {
	t.Helper()
	in := bytes.NewReader(avc)
	out, err := os.Create(filepath.Join(t.TempDir(), "out.mp4"))
	if err != nil {
		t.Fatalf("creating output: %v", err)
	}
	defer func() { _ = out.Close() }()

	result, err := avc2mp4.ConvertFile(in, out)
	if err != nil {
		t.Fatalf("muxing the spooled stream: %v", err)
	}
	return result.FrameCount
}

func TestSpoolWritesTheWholeStreamWhileUnderTheSizeLimit(t *testing.T) {
	stream := bytes.Join(oneSecondOfPictures(5_000_000, 10), nil)
	var file bytes.Buffer
	spool := newAvcSpool(&file, int64(len(stream)))

	if !spoolEverything(spool, stream) {
		t.Fatal("the spool refused a stream that fits under its limit")
	}
	if err := spool.finish(); err != nil {
		t.Fatalf("finish failed: %v", err)
	}

	if !bytes.Equal(file.Bytes(), stream) {
		t.Fatalf("expected every byte on disk, got %d of %d", file.Len(), len(stream))
	}
	if spool.reachedSizeLimit() {
		t.Fatal("a stream under the limit was reported as stopped by it")
	}
}

func TestSpoolStopsBeforeTheUnitThatWouldCrossTheSizeLimit(t *testing.T) {
	units := oneSecondOfPictures(5_000_000, 10)
	fitting := bytes.Join(units[:8], nil) // parameter sets and three whole pictures
	limit := int64(len(fitting) + len(units[8]) - 1)
	var file bytes.Buffer
	spool := newAvcSpool(&file, limit)

	if spoolEverything(spool, bytes.Join(units, nil)) {
		t.Fatal("the spool kept accepting data past its size limit")
	}
	if err := spool.finish(); err != nil {
		t.Fatalf("finish failed: %v", err)
	}

	if !bytes.Equal(file.Bytes(), fitting) {
		t.Fatalf("expected only the %d bytes of whole units that fit, got %d", len(fitting), file.Len())
	}
	if !spool.reachedSizeLimit() {
		t.Fatal("the spool did not report the size limit")
	}
}

func TestSpoolIgnoresFramesAfterTheSizeLimit(t *testing.T) {
	units := oneSecondOfPictures(5_000_000, 10)
	var file bytes.Buffer
	spool := newAvcSpool(&file, int64(len(bytes.Join(units[:4], nil))))
	spoolEverything(spool, bytes.Join(units, nil))
	sizeAtStop := file.Len()

	if spool.write(bytes.Join(oneSecondOfPictures(6_000_000, 10), nil)) {
		t.Fatal("a full spool accepted more frames")
	}
	if file.Len() != sizeAtStop {
		t.Fatalf("file grew from %d to %d bytes after the size limit", sizeAtStop, file.Len())
	}
}

func TestAStreamStoppedBySizeLimitStillMuxesEveryPictureThatFit(t *testing.T) {
	units := oneSecondOfPictures(5_000_000, 10)
	var file bytes.Buffer
	spool := newAvcSpool(&file, int64(len(bytes.Join(units[:12], nil)))) // five whole pictures
	spoolEverything(spool, bytes.Join(units, nil))
	_ = spool.finish()

	if frames := muxedFrameCount(t, file.Bytes()); frames != 5 {
		t.Fatalf("expected the 5 pictures that fit, got %d", frames)
	}
}

func TestSpoolStopsWhenAnUnterminatedUnitOutgrowsTheScannerLimit(t *testing.T) {
	pictures := bytes.Join(oneSecondOfPictures(5_000_000, 10), nil)
	var file bytes.Buffer
	spool := newAvcSpool(&file, avcSizeLimit)
	spoolEverything(spool, pictures)

	// a unit whose closing start code never arrives
	spool.write(withStartCode([]byte{0x41}))
	chunk := bytes.Repeat([]byte{0xFF}, 64<<10)
	accepted := true
	for fed := 0; accepted && fed <= avc2mp4.MaxNALSize; fed += len(chunk) {
		accepted = spool.write(chunk)
	}

	if accepted {
		t.Fatal("the spool kept buffering a unit past the scanner limit")
	}
	if err := spool.finish(); err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	if !bytes.Equal(file.Bytes(), pictures) {
		t.Fatalf("expected only the whole units before the oversized one, got %d of %d bytes", file.Len(), len(pictures))
	}
}
