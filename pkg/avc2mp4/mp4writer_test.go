package avc2mp4

import (
	"bytes"
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

// grouping closes an access unit when the next slice or SPS arrives, so a unit
// is a run of NAL units followed by the SEI carrying its timestamp
func TestGroupAccessUnitsIgnoresNalusBeforeFirstSPS(t *testing.T) {
	// a recording can start mid-stream; everything before the first SPS is
	// undecodable and must be dropped
	units := groupAccessUnits([]NALUnit{
		nalu(1),
		seiWithTimestamp(1_000_000),
		nalu(nalTypeSPS),
		nalu(5),
		seiWithTimestamp(2_000_000),
	})

	if len(units) != 1 {
		t.Fatalf("expected 1 access unit, got %d", len(units))
	}
	if units[0].timestampUs != 2_000_000 {
		t.Errorf("expected timestamp 2000000, got %d", units[0].timestampUs)
	}
}

func TestGroupAccessUnitsSplitsOnEachSlice(t *testing.T) {
	units := groupAccessUnits([]NALUnit{
		nalu(nalTypeSPS),
		nalu(5),
		seiWithTimestamp(1_000_000),
		nalu(1),
		seiWithTimestamp(2_000_000),
		nalu(1),
		seiWithTimestamp(3_000_000),
	})

	if len(units) != 3 {
		t.Fatalf("expected 3 access units, got %d", len(units))
	}
	for i, want := range []uint64{1_000_000, 2_000_000, 3_000_000} {
		if units[i].timestampUs != want {
			t.Errorf("access unit %d: expected timestamp %d, got %d", i, want, units[i].timestampUs)
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

// the timecode SEI is ours, not part of the encoded stream, so it must not be
// muxed into the output
func TestGroupAccessUnitsExcludesTimecodeSEIFromPayload(t *testing.T) {
	units := groupAccessUnits([]NALUnit{
		nalu(nalTypeSPS),
		nalu(5),
		seiWithTimestamp(1_000_000),
		nalu(1),
		seiWithTimestamp(2_000_000),
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
		nalu(5),
		foreignSEI,
		seiWithTimestamp(1_000_000),
		nalu(1),
		seiWithTimestamp(2_000_000),
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
