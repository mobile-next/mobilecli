package avc2mp4

import (
	"encoding/binary"
	"testing"
)

func buildSEINalu(uuid [16]byte, timestampUs uint64) []byte {
	// NAL header (SEI)
	nalu := []byte{0x06}

	// payloadType = 5 (user_data_unregistered)
	nalu = append(nalu, 0x05)

	// payloadSize = 24 (16 UUID + 8 timestamp)
	nalu = append(nalu, 0x18)

	// UUID
	nalu = append(nalu, uuid[:]...)

	// big-endian timestamp
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, timestampUs)
	nalu = append(nalu, ts...)

	// rbsp trailing bits
	nalu = append(nalu, 0x80)

	return nalu
}

func TestParseTimestampValidSEI(t *testing.T) {
	nalu := buildSEINalu(mobilenxTimecodeUUID, 1234567)

	ts, ok := ParseTimestamp(nalu)
	if !ok {
		t.Fatal("expected valid MOBILENXTIMECODE SEI")
	}
	if ts != 1234567 {
		t.Fatalf("expected timestamp 1234567, got %d", ts)
	}
}

func TestParseTimestampLargeValue(t *testing.T) {
	expected := uint64(10_000_000)
	nalu := buildSEINalu(mobilenxTimecodeUUID, expected)

	ts, ok := ParseTimestamp(nalu)
	if !ok {
		t.Fatal("expected valid MOBILENXTIMECODE SEI")
	}
	if ts != expected {
		t.Fatalf("expected timestamp %d, got %d", expected, ts)
	}
}

func TestParseTimestampWrongUUID(t *testing.T) {
	wrongUUID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	nalu := buildSEINalu(wrongUUID, 1234567)

	_, ok := ParseTimestamp(nalu)
	if ok {
		t.Fatal("expected non-matching UUID to be rejected")
	}
}

func TestParseTimestampWrongPayloadType(t *testing.T) {
	nalu := []byte{0x06, 0x01, 0x18}
	nalu = append(nalu, mobilenxTimecodeUUID[:]...)
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, 999)
	nalu = append(nalu, ts...)
	nalu = append(nalu, 0x80)

	_, ok := ParseTimestamp(nalu)
	if ok {
		t.Fatal("expected wrong payload type to be rejected")
	}
}

func TestParseTimestampTooShort(t *testing.T) {
	_, ok := ParseTimestamp([]byte{0x06})
	if ok {
		t.Fatal("expected short data to be rejected")
	}
}

func TestRemoveEmulationPreventionBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "no emulation prevention needed",
			input:    []byte{0x01, 0x02, 0x03, 0x04},
			expected: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			name:     "single emulation prevention byte",
			input:    []byte{0x00, 0x00, 0x03, 0x00},
			expected: []byte{0x00, 0x00, 0x00},
		},
		{
			name:     "emulation prevention before 0x01",
			input:    []byte{0x00, 0x00, 0x03, 0x01},
			expected: []byte{0x00, 0x00, 0x01},
		},
		{
			name:     "emulation prevention before 0x03",
			input:    []byte{0x00, 0x00, 0x03, 0x03},
			expected: []byte{0x00, 0x00, 0x03},
		},
		{
			name:     "multiple emulation prevention bytes",
			input:    []byte{0x00, 0x00, 0x03, 0x00, 0xFF, 0x00, 0x00, 0x03, 0x01},
			expected: []byte{0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x01},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveEmulationPreventionBytes(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(result), len(tt.expected))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Fatalf("byte %d: got 0x%02X, want 0x%02X", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseTimestampWithEmulationPreventionInPayload(t *testing.T) {
	nalu := []byte{0x06}
	nalu = append(nalu, 0x05)
	nalu = append(nalu, 0x18)
	nalu = append(nalu, mobilenxTimecodeUUID[:]...)

	rawTs := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00}
	ebspTs := []byte{0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x01, 0x00}
	nalu = append(nalu, ebspTs...)
	nalu = append(nalu, 0x80)

	ts, ok := ParseTimestamp(nalu)
	if !ok {
		t.Fatal("expected valid MOBILENXTIMECODE SEI with emulation prevention bytes")
	}

	expected := binary.BigEndian.Uint64(rawTs)
	if ts != expected {
		t.Fatalf("expected timestamp %d, got %d", expected, ts)
	}
}
