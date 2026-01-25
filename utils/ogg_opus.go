package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	opusSampleRate = 48000
	opusFrameSize  = 960
	opusPreskip    = 312
)

var oggCRCTable = buildOggCRCTable()

type OggOpusWriter struct {
	w       io.Writer
	serial  uint32
	seq     uint32
	granule uint64
}

func NewOggOpusWriter(w io.Writer) (*OggOpusWriter, error) {
	serial := randomSerial()
	writer := &OggOpusWriter{
		w:      w,
		serial: serial,
	}

	if err := writer.writeOpusHead(); err != nil {
		return nil, err
	}
	if err := writer.writeOpusTags(); err != nil {
		return nil, err
	}
	return writer, nil
}

func (o *OggOpusWriter) WritePacket(packet []byte) error {
	o.granule += opusFrameSize
	return o.writePage(0x00, o.granule, packet)
}

func (o *OggOpusWriter) writeOpusHead() error {
	head := make([]byte, 19)
	copy(head, []byte("OpusHead"))
	head[8] = 1
	head[9] = 1
	binary.LittleEndian.PutUint16(head[10:], opusPreskip)
	binary.LittleEndian.PutUint32(head[12:], opusSampleRate)
	binary.LittleEndian.PutUint16(head[16:], 0)
	head[18] = 0
	return o.writePage(0x02, 0, head)
}

func (o *OggOpusWriter) writeOpusTags() error {
	vendor := []byte("mobilecli")
	packetLen := 8 + 4 + len(vendor) + 4
	packet := make([]byte, packetLen)
	copy(packet, []byte("OpusTags"))
	binary.LittleEndian.PutUint32(packet[8:], uint32(len(vendor)))
	copy(packet[12:], vendor)
	binary.LittleEndian.PutUint32(packet[12+len(vendor):], 0)
	return o.writePage(0x00, 0, packet)
}

func (o *OggOpusWriter) writePage(headerType uint8, granule uint64, packet []byte) error {
	segments := make([]byte, 0, 255)
	remaining := len(packet)
	for remaining > 0 {
		seg := remaining
		if seg > 255 {
			seg = 255
		}
		segments = append(segments, byte(seg))
		remaining -= seg
	}
	if len(segments) > 255 {
		return fmt.Errorf("opus packet too large for single ogg page")
	}

	headerLen := 27 + len(segments)
	page := make([]byte, headerLen+len(packet))
	copy(page[0:4], []byte("OggS"))
	page[4] = 0
	page[5] = headerType
	binary.LittleEndian.PutUint64(page[6:], granule)
	binary.LittleEndian.PutUint32(page[14:], o.serial)
	binary.LittleEndian.PutUint32(page[18:], o.seq)
	binary.LittleEndian.PutUint32(page[22:], 0)
	page[26] = byte(len(segments))
	copy(page[27:], segments)
	copy(page[headerLen:], packet)

	crc := oggCRC(page)
	binary.LittleEndian.PutUint32(page[22:], crc)

	o.seq++
	_, err := o.w.Write(page)
	return err
}

type OpusFrameParser struct {
	buf      []byte
	onPacket func([]byte) error
}

func NewOpusFrameParser(onPacket func([]byte) error) *OpusFrameParser {
	return &OpusFrameParser{
		onPacket: onPacket,
	}
}

func (p *OpusFrameParser) Write(data []byte) error {
	p.buf = append(p.buf, data...)
	for {
		if len(p.buf) < 4 {
			return nil
		}
		packetLen := int(binary.BigEndian.Uint32(p.buf[:4]))
		if packetLen <= 0 {
			p.buf = p.buf[4:]
			continue
		}
		if len(p.buf) < 4+packetLen {
			return nil
		}
		packet := make([]byte, packetLen)
		copy(packet, p.buf[4:4+packetLen])
		p.buf = p.buf[4+packetLen:]
		if err := p.onPacket(packet); err != nil {
			return err
		}
	}
}

func randomSerial() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err == nil {
		return binary.LittleEndian.Uint32(b[:])
	}
	return 0x4f505553
}

func buildOggCRCTable() [256]uint32 {
	var table [256]uint32
	for i := 0; i < 256; i++ {
		r := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if r&0x80000000 != 0 {
				r = (r << 1) ^ 0x04C11DB7
			} else {
				r <<= 1
			}
		}
		table[i] = r
	}
	return table
}

func oggCRC(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc = (crc << 8) ^ oggCRCTable[((crc>>24)&0xFF)^uint32(b)]
	}
	return crc
}

func (o *OggOpusWriter) WriteToBuffer(packet []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := &OggOpusWriter{
		w:       &buf,
		serial:  o.serial,
		seq:     o.seq,
		granule: o.granule,
	}
	if err := writer.WritePacket(packet); err != nil {
		return nil, err
	}
	o.seq = writer.seq
	o.granule = writer.granule
	return buf.Bytes(), nil
}
