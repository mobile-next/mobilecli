package avc2mp4

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"time"

	"github.com/yapingcat/gomedia/go-mp4"
)

// ConvertResult contains the result of an AVC to MP4 conversion
type ConvertResult struct {
	FrameCount int
	Duration   time.Duration
}

type accessUnit struct {
	nalus       []NALUnit
	timestampUs uint64
	hasSlice    bool
}

const (
	nalTypeSEI = 6
	nalTypeSPS = 7
)

// ErrNoFrames means the stream held no picture that could be muxed.
var ErrNoFrames = errors.New("no access units with timestamps found")

// Convert reads raw AVC data and writes a properly-timed MP4 to output.
// returns frame count and duration on success.
func Convert(avcData []byte, output io.WriteSeeker) (*ConvertResult, error) {
	nalus := ParseNALUnits(avcData)
	units := groupAccessUnits(nalus)

	if len(units) == 0 {
		return nil, ErrNoFrames
	}

	if err := writeMp4(units, output); err != nil {
		return nil, err
	}

	timestampsUs := make([]uint64, len(units))
	for i, au := range units {
		timestampsUs[i] = au.timestampUs
	}
	return newConvertResult(timestampsUs), nil
}

// ConvertFile is Convert for a stream on disk. It reads the input twice, once
// for the timestamps and once for the samples, so peak memory is one access
// unit plus a few bytes of sample table per frame, however long the recording.
func ConvertFile(input io.ReadSeeker, output io.WriteSeeker) (*ConvertResult, error) {
	var timestampsUs []uint64
	err := forEachAccessUnit(input, func(au *accessUnit) error {
		timestampsUs = append(timestampsUs, au.timestampUs)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading avc stream: %w", err)
	}
	if len(timestampsUs) == 0 {
		return nil, ErrNoFrames
	}

	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewinding avc stream: %w", err)
	}
	if err := writeMp4Streaming(input, output, timestampsUs); err != nil {
		return nil, err
	}
	return newConvertResult(timestampsUs), nil
}

// forEachAccessUnit reads an Annex B stream and calls fn for every muxable
// access unit, in decode order.
func forEachAccessUnit(input io.Reader, fn func(*accessUnit) error) error {
	var grouper accessUnitGrouper
	err := ScanNALUnits(input, func(nalu NALUnit) error {
		if au := grouper.add(nalu); au != nil {
			return fn(au)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if au := grouper.finish(); au != nil {
		return fn(au)
	}
	return nil
}

// newConvertResult counts the frames and measures the span between the
// earliest and latest timestamp.
func newConvertResult(timestampsUs []uint64) *ConvertResult {
	// units are in decode order, which with B-frames is not presentation order
	firstTs, lastTs := slices.Min(timestampsUs), slices.Max(timestampsUs)
	var duration time.Duration
	const maxDurationMicros = uint64(math.MaxInt64) / uint64(time.Microsecond)
	if delta := lastTs - firstTs; lastTs > firstTs && delta <= maxDurationMicros {
		duration = time.Duration(delta) * time.Microsecond
	}

	return &ConvertResult{
		FrameCount: len(timestampsUs),
		Duration:   duration,
	}
}

// isMuxable reports whether the unit holds a picture and knows when to show it.
func (u *accessUnit) isMuxable() bool {
	return u != nil && u.hasSlice && u.timestampUs > 0
}

// groupAccessUnits splits the stream into one access unit per picture; see
// accessUnitGrouper.
func groupAccessUnits(nalus []NALUnit) []accessUnit {
	var units []accessUnit
	var grouper accessUnitGrouper
	for _, nalu := range nalus {
		if au := grouper.add(nalu); au != nil {
			units = append(units, *au)
		}
	}
	if au := grouper.finish(); au != nil {
		units = append(units, *au)
	}
	return units
}

// accessUnitGrouper builds access units one NAL unit at a time. the encoder
// emits our timecode SEI *before* the picture's slice, so a timestamp is held
// until the next slice claims it. everything before the first SPS is
// undecodable and dropped, as are pictures that never got a timestamp.
type accessUnitGrouper struct {
	current   *accessUnit
	pendingTs uint64
	seenSPS   bool
}

// add feeds the next NAL unit and returns the access unit it completed, if any.
func (g *accessUnitGrouper) add(nalu NALUnit) *accessUnit {
	if nalu.Type == nalTypeSPS {
		g.seenSPS = true
	}
	if !g.seenSPS {
		return nil
	}

	if nalu.Type == nalTypeSEI {
		if ts, ok := ParseTimestamp(nalu.Data); ok {
			g.pendingTs = ts
			return nil // don't include custom SEI in muxed output
		}
	}

	var completed *accessUnit
	isSlice := nalu.Type == 1 || nalu.Type == 5
	// a slice with no fresh timestamp is another slice of the same picture
	isSamePicture := isSlice && g.pendingTs == 0
	if g.current == nil || (g.current.hasSlice && !isSamePicture) {
		completed = g.finish()
		g.current = &accessUnit{}
	}

	g.current.nalus = append(g.current.nalus, nalu)
	if isSlice && !g.current.hasSlice {
		g.current.hasSlice = true
		g.current.timestampUs = g.pendingTs
		g.pendingTs = 0
	}
	return completed
}

// finish returns the access unit still being built, if it can be muxed.
func (g *accessUnitGrouper) finish() *accessUnit {
	au := g.current
	g.current = nil
	if !au.isMuxable() {
		return nil
	}
	return au
}

// sampleTimesMs returns pts and dts in milliseconds for units given in decode
// order. with B-frames the encoder emits pictures out of presentation order, so
// dts cannot simply equal pts: dts walks the sorted timestamps (monotonic), and
// every pts is pushed back by the largest reorder delay so dts <= pts holds.
// without reordering the delay is zero and dts == pts.
func sampleTimesMs(units []accessUnit) (pts []uint64, dts []uint64) {
	timestampsUs := make([]uint64, len(units))
	for i, au := range units {
		timestampsUs[i] = au.timestampUs
	}
	return sampleTimesFromUs(timestampsUs)
}

// sampleTimesFromUs is sampleTimesMs for the bare timestamps, in decode order.
func sampleTimesFromUs(timestampsUs []uint64) (pts []uint64, dts []uint64) {
	dts = slices.Clone(timestampsUs)
	slices.Sort(dts)

	first := dts[0]
	var delayUs uint64
	for i, ts := range timestampsUs {
		if dts[i] > ts {
			delayUs = max(delayUs, dts[i]-ts)
		}
	}

	pts = make([]uint64, len(timestampsUs))
	for i, ts := range timestampsUs {
		pts[i] = (ts - first + delayUs) / 1000
		dts[i] = (dts[i] - first) / 1000
	}
	return pts, dts
}

func writeMp4(units []accessUnit, output io.WriteSeeker) error {
	muxer, trackID, err := newH264Muxer(output)
	if err != nil {
		return err
	}

	pts, dts := sampleTimesMs(units)
	for i, au := range units {
		if err := writeSample(muxer, trackID, &au, pts[i], dts[i]); err != nil {
			return err
		}
	}
	return finishMp4(muxer)
}

// writeMp4Streaming muxes the access units read from input, whose timestamps
// the first pass collected, without holding more than one of them.
func writeMp4Streaming(input io.Reader, output io.WriteSeeker, timestampsUs []uint64) error {
	muxer, trackID, err := newH264Muxer(output)
	if err != nil {
		return err
	}

	pts, dts := sampleTimesFromUs(timestampsUs)
	i := 0
	err = forEachAccessUnit(input, func(au *accessUnit) error {
		if i >= len(pts) {
			return fmt.Errorf("avc stream changed between passes")
		}
		i++
		return writeSample(muxer, trackID, au, pts[i-1], dts[i-1])
	})
	if err != nil {
		return err
	}
	return finishMp4(muxer)
}

func newH264Muxer(output io.WriteSeeker) (*mp4.Movmuxer, uint32, error) {
	muxer, err := mp4.CreateMp4Muxer(output)
	if err != nil {
		return nil, 0, fmt.Errorf("creating mp4 muxer: %w", err)
	}
	return muxer, muxer.AddVideoTrack(mp4.MP4_CODEC_H264), nil
}

func writeSample(muxer *mp4.Movmuxer, trackID uint32, au *accessUnit, pts, dts uint64) error {
	if err := muxer.Write(trackID, buildAnnexB(au.nalus), pts, dts); err != nil {
		return fmt.Errorf("writing frame: %w", err)
	}
	return nil
}

func finishMp4(muxer *mp4.Movmuxer) error {
	if err := muxer.WriteTrailer(); err != nil {
		return fmt.Errorf("writing mp4 trailer: %w", err)
	}
	return nil
}

func buildAnnexB(nalus []NALUnit) []byte {
	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	size := 0
	for _, nalu := range nalus {
		size += 4 + len(nalu.Data)
	}

	buf := make([]byte, 0, size)
	for _, nalu := range nalus {
		buf = append(buf, startCode...)
		buf = append(buf, nalu.Data...)
	}
	return buf
}
