package avc2mp4

import (
	"fmt"
	"io"
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

	firstTs := units[0].timestampUs
	lastTs := units[len(units)-1].timestampUs
	duration := time.Duration(lastTs-firstTs) * time.Microsecond

	return &ConvertResult{
		FrameCount: len(units),
		Duration:   duration,
	}, nil
}

func groupAccessUnits(nalus []NALUnit) []accessUnit {
	var units []accessUnit
	var current *accessUnit
	seenSPS := false

	for _, nalu := range nalus {
		if nalu.Type == nalTypeSPS {
			seenSPS = true
		}
		if !seenSPS {
			continue
		}

		// check if this NAL starts a new access unit
		isSlice := nalu.Type == 1 || nalu.Type == 5
		isSPS := nalu.Type == nalTypeSPS

		if isSlice || isSPS {
			if current != nil && current.timestampUs > 0 {
				units = append(units, *current)
			}
			current = &accessUnit{}
		}

		if current == nil {
			current = &accessUnit{}
		}

		// extract timestamp from our custom SEI
		if nalu.Type == nalTypeSEI {
			if ts, ok := ParseTimestamp(nalu.Data); ok {
				current.timestampUs = ts
				continue // don't include custom SEI in muxed output
			}
		}

		current.nalus = append(current.nalus, nalu)
	}

	// flush last access unit
	if current != nil && current.timestampUs > 0 {
		units = append(units, *current)
	}

	return units
}

func writeMp4(units []accessUnit, output io.WriteSeeker) error {
	muxer, err := mp4.CreateMp4Muxer(output)
	if err != nil {
		return fmt.Errorf("creating mp4 muxer: %w", err)
	}

	trackID := muxer.AddVideoTrack(mp4.MP4_CODEC_H264)

	firstTs := units[0].timestampUs

	for _, au := range units {
		annexB := buildAnnexB(au.nalus)
		ptsMs := (au.timestampUs - firstTs) / 1000
		dtsMs := ptsMs // baseline profile, no B-frames

		err := muxer.Write(trackID, annexB, uint64(ptsMs), uint64(dtsMs))
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
