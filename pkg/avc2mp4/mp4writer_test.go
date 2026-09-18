package avc2mp4

import (
	"bytes"
	"slices"
	"testing"
)

// timestamps only reach the muxer through our custom SEI, so access unit
// grouping is driven by these.
//
// keep test timestamps clear of the 0x00 0x00 0x03 byte pattern: ParseTimestamp
// runs the payload through RemoveEmulationPreventionBytes first, which would
// strip the 0x03 and decode a different number. 1000 (0x…0003E8) is corrupted
// this way; the millions used below are not.
func seiWithTimestamp(timestampUs uint64) NALUnit {
	data := buildSEINalu(mobilenxTimecodeUUID, timestampUs)
	return NALUnit{Type: data[0] & 0x1F, Data: data}
}

func nalu(nalType byte) NALUnit {
	return NALUnit{Type: nalType, Data: []byte{nalType, 0xAA}}
}

// the encoder emits the timecode SEI right before the picture it stamps, so a
// timestamp belongs to the slice that follows it
func timestampsOf(units []accessUnit) []uint64 {
	timestamps := make([]uint64, len(units))
	for i, unit := range units {
		timestamps[i] = unit.timestampUs
	}
	return timestamps
}

func TestGroupAccessUnitsIgnoresNalusBeforeFirstSPS(t *testing.T) {
	// a recording can start mid-stream; everything before the first SPS is
	// undecodable and must be dropped
	units := groupAccessUnits([]NALUnit{
		seiWithTimestamp(1_000_000),
		nalu(1),
		nalu(nalTypeSPS),
		seiWithTimestamp(2_000_000),
		nalu(5),
	})

	if got := timestampsOf(units); !slices.Equal(got, []uint64{2_000_000}) {
		t.Fatalf("expected only the keyframe at 2000000, got %v", got)
	}
}

func TestGroupAccessUnitsGivesEachSliceTheTimestampBeforeIt(t *testing.T) {
	units := groupAccessUnits([]NALUnit{
		nalu(nalTypeSPS),
		seiWithTimestamp(1_000_000),
		nalu(5),
		seiWithTimestamp(2_000_000),
		nalu(1),
		seiWithTimestamp(3_000_000),
		nalu(1),
	})

	if got := timestampsOf(units); !slices.Equal(got, []uint64{1_000_000, 2_000_000, 3_000_000}) {
		t.Fatalf("expected one access unit per slice, got %v", got)
	}
}

func TestGroupAccessUnitsKeepsParameterSetsWithTheirKeyframe(t *testing.T) {
	units := groupAccessUnits([]NALUnit{
		nalu(nalTypeSPS),
		nalu(8),
		seiWithTimestamp(1_000_000),
		nalu(5),
		nalu(nalTypeSPS),
		nalu(8),
		seiWithTimestamp(2_000_000),
		nalu(5),
	})

	if len(units) != 2 {
		t.Fatalf("expected 2 access units, got %d", len(units))
	}
	for i, unit := range units {
		if len(unit.nalus) != 3 || unit.nalus[0].Type != nalTypeSPS || unit.nalus[2].Type != 5 {
			t.Errorf("access unit %d: expected SPS, PPS, IDR together, got %d nalus", i, len(unit.nalus))
		}
	}
}

func TestGroupAccessUnitsDropsUnitsWithoutTimestamp(t *testing.T) {
	units := groupAccessUnits([]NALUnit{
		nalu(nalTypeSPS),
		nalu(5),
		nalu(1),
	})

	if len(units) != 0 {
		t.Fatalf("expected untimestamped units to be dropped, got %d", len(units))
	}
}

// High profile encoders emit B-frames: decode order P B B B while presentation
// order is B B B P. these are real timestamps from a devicekit-ios stream.
func unitsAt(timestampsUs ...uint64) []accessUnit {
	units := make([]accessUnit, len(timestampsUs))
	for i, ts := range timestampsUs {
		units[i] = accessUnit{timestampUs: ts}
	}
	return units
}

func TestSampleTimesStayMonotonicWhenFramesAreReordered(t *testing.T) {
	pts, dts := sampleTimesMs(unitsAt(8700544384, 8700511052, 8700494386, 8700527718, 8700611048))

	if !slices.IsSorted(dts) {
		t.Errorf("dts must never go backwards, got %v", dts)
	}
	for i := range pts {
		if dts[i] > pts[i] {
			t.Errorf("sample %d: dts %d is after pts %d", i, dts[i], pts[i])
		}
	}
	if longest := slices.Max(pts); longest > 1000 {
		t.Errorf("five frames at 60fps should span well under a second, got pts up to %dms", longest)
	}
}

func TestSampleTimesMatchWhenFramesAreInOrder(t *testing.T) {
	pts, dts := sampleTimesMs(unitsAt(5_000_000, 5_016_000, 5_033_000))

	if !slices.Equal(pts, dts) || !slices.Equal(pts, []uint64{0, 16, 33}) {
		t.Errorf("expected pts == dts == [0 16 33], got pts=%v dts=%v", pts, dts)
	}
}

// the timecode SEI is ours, not part of the encoded stream, so it must not be
// muxed into the output
func TestGroupAccessUnitsExcludesTimecodeSEIFromPayload(t *testing.T) {
	units := groupAccessUnits([]NALUnit{
		nalu(nalTypeSPS),
		seiWithTimestamp(1_000_000),
		nalu(5),
		seiWithTimestamp(2_000_000),
		nalu(1),
	})

	if len(units) == 0 {
		t.Fatal("expected at least one access unit")
	}
	for _, unit := range units {
		for _, n := range unit.nalus {
			if n.Type == nalTypeSEI {
				t.Error("timecode SEI should not be part of the muxed payload")
			}
		}
	}
}

// an SEI that is not ours carries no timestamp and belongs in the output
func TestGroupAccessUnitsKeepsForeignSEIInPayload(t *testing.T) {
	foreignSEI := NALUnit{Type: nalTypeSEI, Data: []byte{0x06, 0x01, 0x02, 0x80}}
	units := groupAccessUnits([]NALUnit{
		nalu(nalTypeSPS),
		seiWithTimestamp(1_000_000),
		foreignSEI,
		nalu(5),
		seiWithTimestamp(2_000_000),
		nalu(1),
	})

	found := false
	for _, unit := range units {
		for _, n := range unit.nalus {
			if n.Type == nalTypeSEI {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a non-timecode SEI to survive into the muxed payload")
	}
}

func TestBuildAnnexBPrefixesEveryNalUnit(t *testing.T) {
	out := buildAnnexB([]NALUnit{
		{Type: 7, Data: []byte{0x67, 0xAA}},
		{Type: 5, Data: []byte{0x65, 0xBB}},
	})

	want := []byte{
		0x00, 0x00, 0x00, 0x01, 0x67, 0xAA,
		0x00, 0x00, 0x00, 0x01, 0x65, 0xBB,
	}
	if !bytes.Equal(out, want) {
		t.Fatalf("expected %v, got %v", want, out)
	}
}

func TestBuildAnnexBRoundTripsThroughParseNALUnits(t *testing.T) {
	in := []NALUnit{
		{Type: 7, Data: []byte{0x67, 0xAA, 0xBB}},
		{Type: 5, Data: []byte{0x65, 0xCC}},
	}

	parsed := ParseNALUnits(buildAnnexB(in))
	if len(parsed) != len(in) {
		t.Fatalf("expected %d NAL units back, got %d", len(in), len(parsed))
	}
	for i := range in {
		if !bytes.Equal(parsed[i].Data, in[i].Data) {
			t.Errorf("NAL %d: expected %v, got %v", i, in[i].Data, parsed[i].Data)
		}
	}
}

// Convert rejects these inputs before it ever reaches the muxer, so the tests
// below never write a byte; a stub keeps them off the filesystem
type nopWriteSeeker struct{}

func (nopWriteSeeker) Write(p []byte) (int, error)    { return len(p), nil }
func (nopWriteSeeker) Seek(int64, int) (int64, error) { return 0, nil }

func TestConvertFailsWhenNoTimestampedAccessUnitsExist(t *testing.T) {
	// well-formed NAL units, but none carry a timecode SEI
	data := annexB(startCode4, []byte{0x67, 0xAA}, []byte{0x65, 0xBB})

	if _, err := Convert(data, nopWriteSeeker{}); err == nil {
		t.Fatal("expected an error when no access unit has a timestamp")
	}
}

func TestConvertFailsOnDataWithoutStartCodes(t *testing.T) {
	if _, err := Convert([]byte{0x67, 0xAA, 0xBB}, nopWriteSeeker{}); err == nil {
		t.Fatal("expected an error for data with no start codes")
	}
}
