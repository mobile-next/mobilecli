package avc2mp4

import (
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

// Convert reads raw AVC data and writes a properly-timed MP4 to output.
// returns frame count and duration on success.
func Convert(avcData []byte, output io.WriteSeeker) (*ConvertResult, error) {
	nalus := ParseNALUnits(avcData)
	units := groupAccessUnits(nalus)

	if len(units) == 0 {
		return nil, fmt.Errorf("no access units with timestamps found")
	}

	if err := writeMp4(units, output); err != nil {
		return nil, err
	}

	// units are in decode order, which with B-frames is not presentation order
	firstTs, lastTs := units[0].timestampUs, units[0].timestampUs
	for _, au := range units {
		firstTs, lastTs = min(firstTs, au.timestampUs), max(lastTs, au.timestampUs)
	}
	var duration time.Duration
	const maxDurationMicros = uint64(math.MaxInt64) / uint64(time.Microsecond)
	if delta := lastTs - firstTs; lastTs > firstTs && delta <= maxDurationMicros {
		duration = time.Duration(delta) * time.Microsecond
	}

	return &ConvertResult{
		FrameCount: len(units),
		Duration:   duration,
	}, nil
}

// groupAccessUnits splits the stream into one access unit per picture. the
// encoder emits our timecode SEI *before* the picture's slice, so a timestamp is
// held until the next slice claims it. everything before the first SPS is
// undecodable and dropped, as are pictures that never got a timestamp.
// isMuxable reports whether the unit holds a picture and knows when to show it.
func (u *accessUnit) isMuxable() bool {
	return u != nil && u.hasSlice && u.timestampUs > 0
}

func groupAccessUnits(nalus []NALUnit) []accessUnit {
	var units []accessUnit
	var current *accessUnit
	var pendingTs uint64
	seenSPS := false

	flush := func() {
		if current.isMuxable() {
			units = append(units, *current)
		}
	}

	for _, nalu := range nalus {
		if nalu.Type == nalTypeSPS {
			seenSPS = true
		}
		if !seenSPS {
			continue
		}

		if nalu.Type == nalTypeSEI {
			if ts, ok := ParseTimestamp(nalu.Data); ok {
				pendingTs = ts
				continue // don't include custom SEI in muxed output
			}
		}

		isSlice := nalu.Type == 1 || nalu.Type == 5
		// a slice with no fresh timestamp is another slice of the same picture
		isSamePicture := isSlice && pendingTs == 0
		if current == nil || (current.hasSlice && !isSamePicture) {
			flush()
			current = &accessUnit{}
		}

		current.nalus = append(current.nalus, nalu)
		if isSlice && !current.hasSlice {
			current.hasSlice = true
			current.timestampUs = pendingTs
			pendingTs = 0
		}
	}

	flush()
	return units
}

// sampleTimesMs returns pts and dts in milliseconds for units given in decode
// order. with B-frames the encoder emits pictures out of presentation order, so
// dts cannot simply equal pts: dts walks the sorted timestamps (monotonic), and
// every pts is pushed back by the largest reorder delay so dts <= pts holds.
// without reordering the delay is zero and dts == pts.
func sampleTimesMs(units []accessUnit) (pts []uint64, dts []uint64) {
	dts = make([]uint64, len(units))
	for i, au := range units {
		dts[i] = au.timestampUs
	}
	slices.Sort(dts)

	first := dts[0]
	var delayUs uint64
	for i, au := range units {
		if dts[i] > au.timestampUs {
			delayUs = max(delayUs, dts[i]-au.timestampUs)
		}
	}

	pts = make([]uint64, len(units))
	for i, au := range units {
		pts[i] = (au.timestampUs - first + delayUs) / 1000
		dts[i] = (dts[i] - first) / 1000
	}
	return pts, dts
}

func writeMp4(units []accessUnit, output io.WriteSeeker) error {
	muxer, err := mp4.CreateMp4Muxer(output)
	if err != nil {
		return fmt.Errorf("creating mp4 muxer: %w", err)
	}

	trackID := muxer.AddVideoTrack(mp4.MP4_CODEC_H264)

	pts, dts := sampleTimesMs(units)
	for i, au := range units {
		err := muxer.Write(trackID, buildAnnexB(au.nalus), pts[i], dts[i])
		if err != nil {
			return fmt.Errorf("writing frame: %w", err)
		}
	}

	err = muxer.WriteTrailer()
	if err != nil {
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
