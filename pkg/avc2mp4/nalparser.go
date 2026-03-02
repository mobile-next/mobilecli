package avc2mp4

// NALUnit represents a single H.264 NAL unit
type NALUnit struct {
	Type byte   // nal_unit_type (lower 5 bits of first byte)
	Data []byte // raw NAL unit bytes (including header, without start code)
}

// ParseNALUnits splits an Annex B byte stream into individual NAL units.
// handles both 3-byte (0x00 0x00 0x01) and 4-byte (0x00 0x00 0x00 0x01) start codes.
func ParseNALUnits(data []byte) []NALUnit {
	var units []NALUnit
	n := len(data)
	i := 0

	// find the first start code
	i = findStartCode(data, i)
	if i < 0 {
		return nil
	}

	for i < n {
		// skip past this start code
		if i+3 < n && data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x00 && data[i+3] == 0x01 {
			i += 4
		} else {
			i += 3
		}

		nalStart := i

		// find the next start code (or end of data)
		next := findStartCode(data, i)
		if next < 0 {
			next = n
		}

		nalData := data[nalStart:next]
		if len(nalData) > 0 {
			units = append(units, NALUnit{
				Type: nalData[0] & 0x1F,
				Data: nalData,
			})
		}

		i = next
	}

	return units
}

func findStartCode(data []byte, pos int) int {
	n := len(data)
	for i := pos; i+2 < n; i++ {
		if data[i] == 0x00 && data[i+1] == 0x00 {
			if data[i+2] == 0x01 {
				return i
			}
			if i+3 < n && data[i+2] == 0x00 && data[i+3] == 0x01 {
				return i
			}
		}
	}
	return -1
}
