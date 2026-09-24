package commands

import (
	"io"

	"github.com/mobile-next/mobilecli/pkg/avc2mp4"
)

// avcSizeLimit is the safety stop for one recording's AVC input. The muxed
// file is a little larger than its input, and must stay under the 32-bit MP4
// size boundary (4,294,967,295 bytes); this leaves a wide margin for that.
const avcSizeLimit int64 = 4_000_000_000

// endReasonSizeLimit is reported when a recording hit avcSizeLimit.
const endReasonSizeLimit = "size_limit"

// avcSpool writes a recorded AVC stream to disk one whole NAL unit at a time,
// and stops before the unit that would take the file past its size limit, so
// a stopped file never ends in a torn unit.
type avcSpool struct {
	out      io.Writer
	limit    int64
	written  int64
	splitter avc2mp4.NALSplitter
	full     bool
	err      error
}

func newAvcSpool(out io.Writer, limit int64) *avcSpool {
	return &avcSpool{out: out, limit: limit}
}

// write spools the next chunk. It returns false once capture should stop: the
// size limit was reached or the disk write failed. Later chunks are ignored.
func (s *avcSpool) write(chunk []byte) bool {
	if s.full || s.err != nil {
		return false
	}

	s.splitter.Write(chunk)
	for unit := s.splitter.Next(); unit != nil; unit = s.splitter.Next() {
		if !s.writeUnit(unit) {
			return false
		}
	}
	return true
}

// finish writes the last unit, which no start code closed, unless the size
// limit already ended the recording. It returns the first write error.
func (s *avcSpool) finish() error {
	rest := s.splitter.Flush()
	if rest != nil && !s.full && s.err == nil {
		s.writeUnit(rest)
	}
	return s.err
}

// reachedSizeLimit tells whether the size limit ended the recording.
func (s *avcSpool) reachedSizeLimit() bool {
	return s.full
}

func (s *avcSpool) writeUnit(unit []byte) bool {
	if s.written+int64(len(unit)) > s.limit {
		s.full = true
		return false
	}

	n, err := s.out.Write(unit)
	s.written += int64(n)
	if err != nil {
		s.err = err
		return false
	}
	return true
}
