package avc2mp4

import "encoding/binary"

// UUID for MOBILENXTIMECODE SEI: ASCII bytes of "MOBILENXTIMECODE"
var mobilenxTimecodeUUID = [16]byte{
	0x4D, 0x4F, 0x42, 0x49, 0x4C, 0x45, 0x4E, 0x58, // MOBILENX
	0x54, 0x49, 0x4D, 0x45, 0x43, 0x4F, 0x44, 0x45, // TIMECODE
}

// ParseTimestamp extracts the microsecond timestamp from a MOBILENXTIMECODE SEI NAL unit.
// nalData is the raw NAL unit bytes (starting with the NAL header byte 0x06).
// returns the timestamp in microseconds and true if this is a matching SEI, or 0 and false otherwise.
func ParseTimestamp(nalData []byte) (uint64, bool) {
	if len(nalData) < 2 {
		return 0, false
	}

	// skip NAL header byte (0x06 for SEI)
	rbsp := RemoveEmulationPreventionBytes(nalData[1:])

	pos := 0

	// parse payloadType (could be multi-byte per H.264 spec)
	payloadType := 0
	for pos < len(rbsp) && rbsp[pos] == 0xFF {
		payloadType += 255
		pos++
	}
	if pos >= len(rbsp) {
		return 0, false
	}
	payloadType += int(rbsp[pos])
	pos++

	// user_data_unregistered = 5
	if payloadType != 5 {
		return 0, false
	}

	// parse payloadSize
	payloadSize := 0
	for pos < len(rbsp) && rbsp[pos] == 0xFF {
		payloadSize += 255
		pos++
	}
	if pos >= len(rbsp) {
		return 0, false
	}
	payloadSize += int(rbsp[pos])
	pos++

	// need at least 16 (UUID) + 8 (timestamp) = 24 bytes
	if payloadSize < 24 || pos+24 > len(rbsp) {
		return 0, false
	}

	// check UUID
	var uuid [16]byte
	copy(uuid[:], rbsp[pos:pos+16])
	if uuid != mobilenxTimecodeUUID {
		return 0, false
	}
	pos += 16

	// read big-endian uint64 timestamp in microseconds
	ts := binary.BigEndian.Uint64(rbsp[pos : pos+8])
	return ts, true
}

// RemoveEmulationPreventionBytes strips 0x03 bytes from the pattern 0x00 0x00 0x03
// to recover the original RBSP from EBSP.
func RemoveEmulationPreventionBytes(data []byte) []byte {
	result := make([]byte, 0, len(data))
	n := len(data)
	for i := 0; i < n; i++ {
		if i+2 < n && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x03 {
			result = append(result, 0x00, 0x00)
			i += 2 // skip the 0x03
		} else {
			result = append(result, data[i])
		}
	}
	return result
}
