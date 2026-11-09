package avc2mp4

import (
	"bytes"
	"testing"
	"testing/iotest"
)

func annexB(startCode []byte, payloads ...[]byte) []byte {
	var out []byte
	for _, payload := range payloads {
		out = append(out, startCode...)
		out = append(out, payload...)
	}
	return out
}

var (
	startCode3 = []byte{0x00, 0x00, 0x01}
	startCode4 = []byte{0x00, 0x00, 0x00, 0x01}
)

func TestParseNALUnitsWithThreeByteStartCodes(t *testing.T) {
	// 0x67 = type 7 (SPS), 0x65 = type 5 (IDR slice)
	data := annexB(startCode3, []byte{0x67, 0xAA}, []byte{0x65, 0xBB})

	units := ParseNALUnits(data)
	if len(units) != 2 {
		t.Fatalf("expected 2 NAL units, got %d", len(units))
	}
	if units[0].Type != 7 {
		t.Errorf("expected first NAL type 7, got %d", units[0].Type)
	}
	if units[1].Type != 5 {
		t.Errorf("expected second NAL type 5, got %d", units[1].Type)
	}
}

func TestParseNALUnitsWithFourByteStartCodes(t *testing.T) {
	data := annexB(startCode4, []byte{0x67, 0xAA}, []byte{0x68, 0xBB}, []byte{0x65, 0xCC})

	units := ParseNALUnits(data)
	if len(units) != 3 {
		t.Fatalf("expected 3 NAL units, got %d", len(units))
	}
	if units[1].Type != 8 {
		t.Errorf("expected second NAL type 8 (PPS), got %d", units[1].Type)
	}
}

func TestParseNALUnitsWithMixedStartCodeLengths(t *testing.T) {
	var data []byte
	data = append(data, startCode4...)
	data = append(data, 0x67, 0xAA)
	data = append(data, startCode3...)
	data = append(data, 0x65, 0xBB, 0xCC)

	units := ParseNALUnits(data)
	if len(units) != 2 {
		t.Fatalf("expected 2 NAL units, got %d", len(units))
	}
	if len(units[1].Data) != 3 {
		t.Errorf("expected trailing NAL to keep all 3 bytes, got %d", len(units[1].Data))
	}
}

// a nal header is forbidden_zero_bit(1) | nal_ref_idc(2) | nal_unit_type(5), so
// 0x45 is ref_idc=2 type=5 — a valid header that still exercises the masking
func TestParseNALUnitsMasksRefIdcOutOfType(t *testing.T) {
	data := annexB(startCode4, []byte{0x45})

	units := ParseNALUnits(data)
	if len(units) != 1 {
		t.Fatalf("expected 1 NAL unit, got %d", len(units))
	}
	if units[0].Type != 5 {
		t.Errorf("expected type 5 from header 0x45, got %d", units[0].Type)
	}
	if units[0].Data[0] != 0x45 {
		t.Errorf("expected Data to retain the original header byte, got 0x%02X", units[0].Data[0])
	}
}

func TestParseNALUnitsReturnsNilWithoutStartCode(t *testing.T) {
	if units := ParseNALUnits([]byte{0x67, 0xAA, 0xBB}); units != nil {
		t.Fatalf("expected nil for data with no start code, got %d units", len(units))
	}
}

func TestParseNALUnitsReturnsNilForEmptyInput(t *testing.T) {
	if units := ParseNALUnits(nil); units != nil {
		t.Fatalf("expected nil for empty input, got %d units", len(units))
	}
}

// a stream ending on a bare start code has no payload to report
func TestParseNALUnitsSkipsEmptyTrailingUnit(t *testing.T) {
	data := annexB(startCode4, []byte{0x67, 0xAA})
	data = append(data, startCode4...)

	units := ParseNALUnits(data)
	if len(units) != 1 {
		t.Fatalf("expected 1 NAL unit, got %d", len(units))
	}
}

// a stream with both start code lengths, a zero byte right before a 4-byte
// start code, and a trailing unit that no start code closes
func mixedStream() []byte {
	var data []byte
	data = append(data, 0xEE) // garbage before the first start code is dropped
	data = append(data, startCode4...)
	data = append(data, 0x67, 0xAA, 0x00)
	data = append(data, startCode3...)
	data = append(data, 0x68, 0xBB)
	data = append(data, startCode4...)
	data = append(data, 0x65, 0x88, 0x00, 0x00, 0x03, 0x01)
	data = append(data, startCode3...)
	data = append(data, 0x41, 0x9A)
	return data
}

func splitOneByteAtATime(data []byte) [][]byte {
	var splitter NALSplitter
	var units [][]byte
	for _, b := range data {
		splitter.Write([]byte{b})
		for unit := splitter.Next(); unit != nil; unit = splitter.Next() {
			units = append(units, unit)
		}
	}
	if rest := splitter.Flush(); rest != nil {
		units = append(units, rest)
	}
	return units
}

func payloadsOf(units []NALUnit) [][]byte {
	payloads := make([][]byte, len(units))
	for i, unit := range units {
		payloads[i] = unit.Data
	}
	return payloads
}

func withoutStartCode(unit []byte) []byte {
	if unit[2] == 0x01 {
		return unit[3:]
	}
	return unit[4:]
}

func TestNALSplitterFindsTheSameUnitsAsParseNALUnitsWhenFedByteByByte(t *testing.T) {
	want := payloadsOf(ParseNALUnits(mixedStream()))

	units := splitOneByteAtATime(mixedStream())
	if len(units) != len(want) {
		t.Fatalf("expected %d units, got %d", len(want), len(units))
	}
	for i, unit := range units {
		if !bytes.Equal(withoutStartCode(unit), want[i]) {
			t.Errorf("unit %d: expected %x, got %x", i, want[i], withoutStartCode(unit))
		}
	}
}

func TestNALSplitterKeepsItsStartCodes(t *testing.T) {
	var splitter NALSplitter
	splitter.Write(annexB(startCode4, []byte{0x67, 0xAA}, []byte{0x68, 0xBB}))

	if unit := splitter.Next(); !bytes.Equal(unit, annexB(startCode4, []byte{0x67, 0xAA})) {
		t.Fatalf("expected the SPS with its start code, got %x", unit)
	}
}

func TestNALSplitterHoldsAUnitBackUntilTheNextStartCodeArrives(t *testing.T) {
	var splitter NALSplitter
	splitter.Write(annexB(startCode4, []byte{0x65, 0x88, 0x99}))

	if unit := splitter.Next(); unit != nil {
		t.Fatalf("a unit with no start code after it may still grow, got %x", unit)
	}

	splitter.Write(startCode4)
	if unit := splitter.Next(); !bytes.Equal(unit, annexB(startCode4, []byte{0x65, 0x88, 0x99})) {
		t.Fatalf("expected the finished unit once the next start code arrived, got %x", unit)
	}
}

func TestScanNALUnitsReadsTheSameUnitsAsParseNALUnits(t *testing.T) {
	want := payloadsOf(ParseNALUnits(mixedStream()))

	var got [][]byte
	err := ScanNALUnits(iotest.OneByteReader(bytes.NewReader(mixedStream())), func(unit NALUnit) error {
		got = append(got, unit.Data)
		return nil
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d units, got %d", len(want), len(got))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("unit %d: expected %x, got %x", i, want[i], got[i])
		}
	}
}
