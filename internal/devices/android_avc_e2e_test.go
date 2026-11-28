package devices

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mobile-next/mobilecli/internal/avc2mp4"
	"github.com/stretchr/testify/require"
)

// an Android encoder's output, buffer by buffer: the codec config buffer
// (SPS/PPS) once, then one buffer per picture with AvcServer's timecode SEI in
// front of its slice, a key frame every second and no parameter sets with it
var (
	androidSPS = annexB(0x67, 0x64, 0x00, 0x0A, 0xAC, 0x72, 0x84, 0x44, 0x26, 0x84, 0x00, 0x00,
		0x03, 0x00, 0x04, 0x00, 0x00, 0x03, 0x00, 0xCA, 0x3C, 0x48, 0x96, 0x11, 0x80)
	androidPPS = annexB(0x68, 0xE8, 0x43, 0x8F, 0x13, 0x21, 0x30)
)

const androidFramesPerSecond = 30

func annexB(unit ...byte) []byte { return append([]byte{0, 0, 0, 1}, unit...) }

func androidTimecode(timestampUs uint64) []byte {
	payload := append([]byte{0x06, 0x05, 24}, []byte("MOBILENXTIMECODE")...)
	payload = binary.BigEndian.AppendUint64(payload, timestampUs)
	return annexB(append(payload, 0x80)...)
}

func androidPicture(index int, timestampUs uint64) []byte {
	slice := annexB(0x41, 0x9A, byte(index), 0xFF)
	if index%androidFramesPerSecond == 0 {
		slice = annexB(0x65, 0x88, byte(index), 0xFF)
	}
	return concat(androidTimecode(timestampUs), slice)
}

// androidPictures returns pictures [from, to) at 30 fps starting at 5s.
func androidPictures(from, to int) []byte {
	var out []byte
	for i := from; i < to; i++ {
		out = append(out, androidPicture(i, 5_000_000+uint64(i)*1_000_000/androidFramesPerSecond)...)
	}
	return out
}

func muxCapturedBytes(t *testing.T, avc []byte) *avc2mp4.ConvertResult {
	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), "recording.mp4"))
	require.NoError(t, err)
	defer func() { _ = out.Close() }()

	result, err := avc2mp4.ConvertFile(bytes.NewReader(avc), out)
	require.NoError(t, err)
	return result
}

func TestARecordingThatStartsTheAndroidStreamMuxesEveryPicture(t *testing.T) {
	launcher := newFakeAvcLauncher()
	hub := &avcHub{}
	recording := startCapture(hub, sharedAndroidSource(launcher, time.Second))
	server := launcher.waitForLaunch(t)

	stream := concat(androidSPS, androidPPS, androidPictures(0, 61))
	server.print(stream)
	recording.waitForBytes(t, len(stream))
	recording.interrupt()
	require.NoError(t, recording.waitUntilFinished(t))

	result := muxCapturedBytes(t, recording.bytes())
	require.Equal(t, 61, result.FrameCount)
	require.Equal(t, 2*time.Second, result.Duration)
}

func TestARecordingThatJoinsALiveAndroidStreamStartsDecodableAtTheNextKeyFrame(t *testing.T) {
	launcher := newFakeAvcLauncher()
	hub := &avcHub{}
	// this source answers the joiner's key frame request with nothing, so the
	// joiner starts at the encoder's own next key frame (picture 30)
	src := sharedAndroidSource(launcher, time.Second)
	// The hub asks for a key frame only once the joiner is registered, so the
	// first request also says the recording is attached.
	keyFrameAsked := make(chan struct{})
	var askedOnce sync.Once
	src.requestKeyFrame = func() error {
		askedOnce.Do(func() { close(keyFrameAsked) })
		return nil
	}
	live := startCapture(hub, src)
	server := launcher.waitForLaunch(t)
	server.print(concat(androidSPS, androidPPS, androidPictures(0, 10)))
	live.waitForBytes(t, 1)

	recording := startCapture(hub, src)
	select {
	case <-keyFrameAsked:
	case <-time.After(2 * time.Second):
		t.Fatal("the joining recording never asked for a key frame")
	}
	server.print(androidPictures(10, 91))
	recording.waitForBytes(t, len(androidSPS)+len(androidPPS)+len(androidPictures(30, 91))-len(androidPicture(90, 0)))
	recording.interrupt()
	require.NoError(t, recording.waitUntilFinished(t))
	live.interrupt()
	require.NoError(t, live.waitUntilFinished(t))

	got := recording.bytes()
	require.True(t, bytes.HasPrefix(got, concat(androidSPS, androidPPS, androidTimecode(5_000_000+30*1_000_000/androidFramesPerSecond))),
		"the joiner must start with the cached SPS/PPS and the key frame's timecode")
	result := muxCapturedBytes(t, got)
	require.Equal(t, 61, result.FrameCount, "pictures 30..90")
	require.Equal(t, 2*time.Second, result.Duration)
}
