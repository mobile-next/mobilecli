package machomap

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

// HostFilesResolver resolves symbols from on-disk Xcode device-symbol Mach-O
// files plus the single shared-cache slide read from the device. This is the
// fast Mac-only resolver used to prove the Go mapper matches the Python
// prototype; AWS Device Farm uses the host-independent export-trie resolver.
type HostFilesResolver struct {
	c         *RSP
	files     []string
	slide     uint64
	haveSlide bool
}

// NewHostFilesResolver builds a resolver over the standard device-symbol layout
// under symbolRoot (…/Symbols/usr/lib).
func NewHostFilesResolver(c *RSP, symbolRoot string) *HostFilesResolver {
	return &HostFilesResolver{
		c: c,
		files: []string{
			symbolRoot + "/system/libsystem_kernel.dylib",
			symbolRoot + "/system/libdyld.dylib",
			symbolRoot + "/libobjc.A.dylib",
		},
	}
}

// Resolve returns runtime addresses for names, searching each symbol file's
// LC_SYMTAB for "_name" and adding the shared-cache slide.
func (r *HostFilesResolver) Resolve(names []string) (map[string]uint64, error) {
	if !r.haveSlide {
		slide, err := readSharedCacheSlide(r.c)
		if err != nil {
			return nil, err
		}
		r.slide, r.haveSlide = slide, true
	}
	want := map[string]bool{}
	for _, n := range names {
		want["_"+n] = true
	}
	found := map[string]uint64{}
	for _, f := range r.files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue // not every name lives in every file
		}
		scanSymtab(data, want, func(name string, value uint64) {
			found[strings.TrimPrefix(name, "_")] = value + r.slide
		})
	}
	var missing []string
	for _, n := range names {
		if _, ok := found[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("symbols not found in host symbol files: %v", missing)
	}
	return found, nil
}

// readSharedCacheSlide reads dyld_all_image_infos and returns the shared-cache
// slide (offset 152), requiring dyld info version >= 15 and libSystemInitialized.
func readSharedCacheSlide(c *RSP) (uint64, error) {
	resp, err := c.Request("qShlibInfoAddr")
	if err != nil {
		return 0, err
	}
	var addr uint64
	if _, err := fmt.Sscanf(resp, "%x", &addr); err != nil {
		return 0, fmt.Errorf("qShlibInfoAddr %q: %w", resp, err)
	}
	info, err := c.Memory(addr, 184)
	if err != nil {
		return 0, err
	}
	if binary.LittleEndian.Uint32(info) < 15 {
		return 0, fmt.Errorf("unsupported dyld info layout")
	}
	if info[25] == 0 {
		return 0, fmt.Errorf("libSystem not initialized yet")
	}
	return binary.LittleEndian.Uint64(info[152:]), nil
}

// scanSymtab walks a thin Mach-O's LC_SYMTAB and calls hit(name, value) for each
// wanted symbol with a nonzero value.
func scanSymtab(data []byte, want map[string]bool, hit func(name string, value uint64)) {
	if len(data) < 32 || binary.LittleEndian.Uint32(data) != machoMagic64 {
		return
	}
	ncmds := binary.LittleEndian.Uint32(data[16:])
	off := 32
	for i := uint32(0); i < ncmds; i++ {
		if off+8 > len(data) {
			return
		}
		cmd := binary.LittleEndian.Uint32(data[off:])
		size := binary.LittleEndian.Uint32(data[off+4:])
		if size == 0 {
			return
		}
		if cmd == lcSymtab {
			symoff := binary.LittleEndian.Uint32(data[off+8:])
			nsyms := binary.LittleEndian.Uint32(data[off+12:])
			stroff := binary.LittleEndian.Uint32(data[off+16:])
			for n := uint32(0); n < nsyms; n++ {
				base := int(symoff) + int(n)*16
				if base+16 > len(data) {
					break
				}
				strx := binary.LittleEndian.Uint32(data[base:])
				value := binary.LittleEndian.Uint64(data[base+8:])
				if value == 0 {
					continue
				}
				name := cstrAt(data, int(stroff)+int(strx), len(data))
				if want[name] {
					hit(name, value)
				}
			}
		}
		off += int(size)
	}
}
