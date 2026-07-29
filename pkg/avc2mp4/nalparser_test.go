package avc2mp4

import "testing"

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

// the type lives in the low 5 bits; the two high bits are nal_ref_idc and must
// not leak into the reported type
func TestParseNALUnitsMasksRefIdcOutOfType(t *testing.T) {
	data := annexB(startCode4, []byte{0xE5})

	units := ParseNALUnits(data)
	if len(units) != 1 {
		t.Fatalf("expected 1 NAL unit, got %d", len(units))
	}
	if units[0].Type != 5 {
		t.Errorf("expected type 5 from header 0xE5, got %d", units[0].Type)
	}
	if units[0].Data[0] != 0xE5 {
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
