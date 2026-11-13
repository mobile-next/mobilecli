// Package machomap maps a precompiled arm64 Mach-O dylib into a running
// get-task-allow process over the on-device debugserver, with no code signing
// and no re-signing of the dylib or the app. The CS_DEBUGGED exemption (granted
// to a process an attached debugger controls) lets the debugger execute pages it
// wrote itself, so we can map, fix up, and run an unsigned dylib the same way a
// native loader would — the primitive mobilecli needs to run its instrumentation
// agent on real devices and on AWS Device Farm, where dlopen is impossible.
//
// This file parses the thin arm64 Mach-O: segments, symbols, sections, the
// offset-based initializer sections, and the offset of the chained-fixups blob.
// It is a direct port of the proven Python prototype (inject_map.py).
package machomap

import (
	"encoding/binary"
	"fmt"
)

const (
	machoMagic64        = 0xFEEDFACF
	lcSegment64         = 0x19
	lcSymtab            = 0x02
	lcDyldChainedFixups = 0x80000034
	sInitFuncOffsets    = 0x16 // section flags low byte: offsets (not pointers) to initializers
)

// Segment is one LC_SEGMENT_64. InitProt's write bit decides which segments are
// remapped read-write after the whole image is reserved executable.
type Segment struct {
	Name     string
	VMAddr   uint64
	VMSize   uint64
	FileOff  uint64
	FileSize uint64
	InitProt int32
}

// Section records a section's vmaddr and size, keyed by name in Macho.Sections
// (e.g. __objc_classlist, __objc_selrefs, __objc_imageinfo).
type Section struct {
	Addr uint64
	Size uint64
}

// InitSection is an S_INIT_FUNC_OFFSETS section: each uint32 is an offset from
// the image base to an initializer (the agent's C constructor lives here).
type InitSection struct {
	Addr uint64
	Size uint64
}

// Macho is the parsed subset of a thin arm64 Mach-O the mapper needs.
type Macho struct {
	Data         []byte
	Segments     []Segment
	Symbols      map[string]uint64 // name -> n_value (unslid)
	Sections     map[string]Section
	InitSections []InitSection
	ChainedOff   uint32 // file offset of the LC_DYLD_CHAINED_FIXUPS blob, 0 if none
	HasChained   bool
}

// ParseMacho parses a thin arm64 64-bit Mach-O image.
func ParseMacho(data []byte) (*Macho, error) {
	if len(data) < 32 || binary.LittleEndian.Uint32(data) != machoMagic64 {
		return nil, fmt.Errorf("expected a thin arm64 64-bit Mach-O")
	}
	m := &Macho{
		Data:     data,
		Symbols:  map[string]uint64{},
		Sections: map[string]Section{},
	}
	ncmds := binary.LittleEndian.Uint32(data[16:])
	off := 32
	for i := uint32(0); i < ncmds; i++ {
		if off+8 > len(data) {
			return nil, fmt.Errorf("load command %d out of range", i)
		}
		cmd := binary.LittleEndian.Uint32(data[off:])
		size := binary.LittleEndian.Uint32(data[off+4:])
		if size == 0 || off+int(size) > len(data) {
			return nil, fmt.Errorf("bad load command size at %d", off)
		}
		switch cmd {
		case lcDyldChainedFixups:
			m.ChainedOff = binary.LittleEndian.Uint32(data[off+8:])
			m.HasChained = true
		case lcSegment64:
			seg := Segment{
				Name:     cstr16(data[off+8 : off+24]),
				VMAddr:   binary.LittleEndian.Uint64(data[off+24:]),
				VMSize:   binary.LittleEndian.Uint64(data[off+32:]),
				FileOff:  binary.LittleEndian.Uint64(data[off+40:]),
				FileSize: binary.LittleEndian.Uint64(data[off+48:]),
				InitProt: int32(binary.LittleEndian.Uint32(data[off+60:])), // #nosec G115 -- vm_prot_t is a small flag set
			}
			m.Segments = append(m.Segments, seg)
			nsects := binary.LittleEndian.Uint32(data[off+64:])
			soff := off + 72
			for s := uint32(0); s < nsects; s++ {
				sectname := cstr16(data[soff : soff+16])
				addr := binary.LittleEndian.Uint64(data[soff+32:])
				ssize := binary.LittleEndian.Uint64(data[soff+40:])
				flags := binary.LittleEndian.Uint32(data[soff+64:])
				if flags&0xFF == sInitFuncOffsets {
					m.InitSections = append(m.InitSections, InitSection{Addr: addr, Size: ssize})
				}
				m.Sections[sectname] = Section{Addr: addr, Size: ssize}
				soff += 80
			}
		case lcSymtab:
			symoff := binary.LittleEndian.Uint32(data[off+8:])
			nsyms := binary.LittleEndian.Uint32(data[off+12:])
			stroff := binary.LittleEndian.Uint32(data[off+16:])
			strsize := binary.LittleEndian.Uint32(data[off+20:])
			for n := uint32(0); n < nsyms; n++ {
				base := int(symoff) + int(n)*16
				if base+16 > len(data) {
					break
				}
				strx := binary.LittleEndian.Uint32(data[base:])
				value := binary.LittleEndian.Uint64(data[base+8:])
				name := cstrAt(data, int(stroff)+int(strx), int(stroff)+int(strsize))
				if name != "" {
					m.Symbols[name] = value
				}
			}
		}
		off += int(size)
	}
	return m, nil
}

// BuildImage lays the mapped segments out contiguously at their vmaddrs
// (__LINKEDIT is not mapped) and returns the image buffer, its size, and the
// mapped segments.
func (m *Macho) BuildImage() (image []byte, imageSize uint64, mapped []Segment) {
	for _, s := range m.Segments {
		if s.Name == "__LINKEDIT" {
			continue
		}
		mapped = append(mapped, s)
		if s.VMAddr+s.VMSize > imageSize {
			imageSize = s.VMAddr + s.VMSize
		}
	}
	image = make([]byte, imageSize)
	for _, s := range mapped {
		copy(image[s.VMAddr:s.VMAddr+s.FileSize], m.Data[s.FileOff:s.FileOff+s.FileSize])
	}
	return image, imageSize, mapped
}

// SectionCount returns the number of 8-byte pointer slots in a section (0 if
// the section is absent) — used for __objc_classlist / __objc_selrefs / etc.
func (m *Macho) SectionCount(name string) uint64 {
	if s, ok := m.Sections[name]; ok {
		return s.Size / 8
	}
	return 0
}

func cstr16(b []byte) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return string(b[:n])
}

func cstrAt(data []byte, start, limit int) string {
	if start < 0 || start >= len(data) || start >= limit {
		return ""
	}
	end := start
	for end < limit && end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[start:end])
}
