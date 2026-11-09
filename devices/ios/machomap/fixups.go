package machomap

import (
	"encoding/binary"
	"fmt"
)

// Chained-fixup pointer formats we handle. Both reduce to "runtime address =
// load_base + target" because the dylib's preferred base is 0 (arm64, non-arm64e).
const (
	dyldChainedPtr64       = 2 // DYLD_CHAINED_PTR_64
	dyldChainedPtr64Offset = 6 // DYLD_CHAINED_PTR_64_OFFSET
	chainedPtrStartNone    = 0xFFFF

	dyldChainedImport              = 1 // DYLD_CHAINED_IMPORT
	dyldChainedSymbolsUncompressed = 0
)

// fixupSeg is one dyld_chained_starts_in_segment: where a segment's fixup chains
// begin and how to stride them.
type fixupSeg struct {
	pageSize      uint32
	ptrFormat     uint16
	segmentOffset uint64
	pageStarts    []uint16
}

// requireSupportedFixupFormats refuses layouts the parser would misread: wrong
// names mean wrong pointers written into the live app. A toolchain change can
// select another format with no source change.
func requireSupportedFixupFormats(header []byte) error {
	if format := binary.LittleEndian.Uint32(header[20:]); format != dyldChainedImport {
		return fmt.Errorf("unsupported chained imports format %d", format)
	}
	if format := binary.LittleEndian.Uint32(header[24:]); format != dyldChainedSymbolsUncompressed {
		return fmt.Errorf("unsupported chained symbols format %d", format)
	}
	return nil
}

// ParseChainedFixups returns the imported symbol names (in ordinal order), a
// parallel slice marking weak imports (which may legitimately resolve to 0), and
// the per-segment chain-start tables from the LC_DYLD_CHAINED_FIXUPS blob at
// hdrOff.
func ParseChainedFixups(data []byte, hdrOff uint32) (imports []string, weak []bool, segs []fixupSeg, err error) {
	h := int(hdrOff)
	if h+28 > len(data) {
		return nil, nil, nil, fmt.Errorf("chained-fixups header out of range")
	}
	startsOff := binary.LittleEndian.Uint32(data[h+4:])
	impOff := binary.LittleEndian.Uint32(data[h+8:])
	symOff := binary.LittleEndian.Uint32(data[h+12:])
	impCnt := binary.LittleEndian.Uint32(data[h+16:])
	if err := requireSupportedFixupFormats(data[h:]); err != nil {
		return nil, nil, nil, err
	}
	// imports format 1: {lib_ordinal:8, weak_import:1, name_offset:23}
	for i := uint32(0); i < impCnt; i++ {
		v := binary.LittleEndian.Uint32(data[h+int(impOff)+int(i)*4:])
		nameOff := (v >> 9) & 0x7FFFFF
		name := cstrAt(data, h+int(symOff)+int(nameOff), len(data))
		imports = append(imports, name)
		weak = append(weak, (v>>8)&1 == 1)
	}
	segCount := binary.LittleEndian.Uint32(data[h+int(startsOff):])
	for s := uint32(0); s < segCount; s++ {
		so := binary.LittleEndian.Uint32(data[h+int(startsOff)+4+int(s)*4:])
		if so == 0 {
			continue
		}
		b := h + int(startsOff) + int(so)
		pageSize := binary.LittleEndian.Uint16(data[b+4:])
		ptrFormat := binary.LittleEndian.Uint16(data[b+6:])
		segmentOffset := binary.LittleEndian.Uint64(data[b+8:])
		pageCount := binary.LittleEndian.Uint16(data[b+20:])
		starts := make([]uint16, pageCount)
		for p := 0; p < int(pageCount); p++ {
			starts[p] = binary.LittleEndian.Uint16(data[b+22+p*2:])
		}
		segs = append(segs, fixupSeg{
			pageSize:      uint32(pageSize),
			ptrFormat:     ptrFormat,
			segmentOffset: segmentOffset,
			pageStarts:    starts,
		})
	}
	return imports, weak, segs, nil
}

// ApplyFixups patches the image buffer in place: rebases -> base+target+(high8<<56),
// binds -> importAddrs[ordinal]+addend. Each slot is read before it is overwritten
// so the chain's next-stride stays valid. Returns (rebases, binds).
func ApplyFixups(image []byte, segs []fixupSeg, base uint64, importAddrs []uint64) (int, int, error) {
	rebases, binds := 0, 0
	for _, seg := range segs {
		if seg.ptrFormat != dyldChainedPtr64 && seg.ptrFormat != dyldChainedPtr64Offset {
			return 0, 0, fmt.Errorf("unsupported chained pointer format %d", seg.ptrFormat)
		}
		for pageIndex, start := range seg.pageStarts {
			if start == chainedPtrStartNone {
				continue
			}
			cur := seg.segmentOffset + uint64(pageIndex)*uint64(seg.pageSize) + uint64(start)
			for {
				if cur+8 > uint64(len(image)) {
					return 0, 0, fmt.Errorf("fixup slot %#x out of image", cur)
				}
				v := binary.LittleEndian.Uint64(image[cur:])
				isBind := (v >> 63) & 1
				next := (v >> 51) & 0xFFF // stride to next fixup, in 4-byte units
				var newval uint64
				if isBind == 1 { // {ordinal:24, addend:8, reserved:19, next:12, bind:1}
					ordinal := v & 0xFFFFFF
					addend := (v >> 24) & 0xFF
					if ordinal >= uint64(len(importAddrs)) {
						return 0, 0, fmt.Errorf("bind ordinal %d out of range", ordinal)
					}
					newval = importAddrs[ordinal] + addend
					binds++
				} else { // {target:36, high8:8, reserved:7, next:12, bind:1}
					target := v & 0xFFFFFFFFF
					high8 := (v >> 36) & 0xFF
					newval = base + target + (high8 << 56)
					rebases++
				}
				binary.LittleEndian.PutUint64(image[cur:], newval)
				if next == 0 {
					break
				}
				cur += next * 4
			}
		}
	}
	return rebases, binds, nil
}
