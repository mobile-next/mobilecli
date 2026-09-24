package avc2mp4

import (
	"errors"
	"fmt"
	"io"
)

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
	i = FindStartCode(data, i)
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
		next := FindStartCode(data, i)
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

// FindStartCode returns the index of the first Annex B start code at or after
// pos, or -1 when data holds none from there on.
func FindStartCode(data []byte, pos int) int {
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

// scanChunkSize is how much ScanNALUnits reads at a time.
const scanChunkSize = 64 << 10

// maxNALSize bounds one NAL unit while scanning, so a file without start codes
// cannot be pulled into memory whole.
const maxNALSize = 64 << 20

// NALSplitter cuts an Annex B stream that arrives in arbitrary chunks into
// whole NAL units, holding back only the unit that is still incomplete. It
// finds the same units as ParseNALUnits would on the whole stream.
type NALSplitter struct {
	buf      []byte
	resumeAt int // where the search for the end of the first unit resumes
}

// Write appends the next chunk of the stream.
func (s *NALSplitter) Write(chunk []byte) {
	s.buf = append(s.buf, chunk...)
}

// Next returns the next complete NAL unit, with its start code, or nil when
// the buffered bytes do not finish one yet. The returned slice is never
// written to again.
func (s *NALSplitter) Next() []byte {
	start := FindStartCode(s.buf, 0)
	if start < 0 {
		// nothing decodable yet; keep only what could begin a start code
		s.buf = s.buf[max(len(s.buf)-3, 0):]
		s.resumeAt = 0
		return nil
	}
	if start > 0 {
		s.buf = s.buf[start:]
		s.resumeAt = 0
	}

	body := startCodeLength(s.buf)
	end := FindStartCode(s.buf, max(body, s.resumeAt))
	if end < 0 {
		// a start code can straddle the next chunk, so look back 3 bytes
		s.resumeAt = max(body, len(s.buf)-3)
		return nil
	}

	unit := s.buf[:end:end]
	s.buf = s.buf[end:]
	s.resumeAt = 0
	return unit
}

// Flush returns what is left at the end of the stream: the last unit, which
// no start code closed, or nil when there is none.
func (s *NALSplitter) Flush() []byte {
	rest := s.buf
	s.buf, s.resumeAt = nil, 0
	if FindStartCode(rest, 0) != 0 || len(rest) <= startCodeLength(rest) {
		return nil
	}
	return rest
}

// Buffered reports how many bytes are held back for the next unit.
func (s *NALSplitter) Buffered() int {
	return len(s.buf)
}

// startCodeLength returns the length of the start code data begins with.
func startCodeLength(data []byte) int {
	if data[2] == 0x01 {
		return 3
	}
	return 4
}

// ScanNALUnits reads an Annex B stream from r and calls fn for every NAL unit
// in order, holding at most one unit in memory. Data is a fresh copy.
func ScanNALUnits(r io.Reader, fn func(NALUnit) error) error {
	var splitter NALSplitter
	chunk := make([]byte, scanChunkSize)

	for {
		n, readErr := r.Read(chunk)
		splitter.Write(chunk[:n])
		for unit := splitter.Next(); unit != nil; unit = splitter.Next() {
			if err := emitNALUnit(unit, fn); err != nil {
				return err
			}
		}
		if splitter.Buffered() > maxNALSize {
			return fmt.Errorf("NAL unit larger than %d bytes", maxNALSize)
		}

		if errors.Is(readErr, io.EOF) {
			return emitNALUnit(splitter.Flush(), fn)
		}
		if readErr != nil {
			return readErr
		}
	}
}

// emitNALUnit hands one raw unit (with its start code) to fn, skipping units
// without a payload the way ParseNALUnits does.
func emitNALUnit(raw []byte, fn func(NALUnit) error) error {
	if raw == nil {
		return nil
	}
	data := raw[startCodeLength(raw):]
	if len(data) == 0 {
		return nil
	}
	return fn(NALUnit{Type: data[0] & 0x1F, Data: data})
}
