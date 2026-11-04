package machomap

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

var trieDebug = os.Getenv("MACHOMAP_DEBUG") != ""

// ExportTrieResolver resolves symbols with no host symbol files: it finds the
// relevant shared-cache dylibs in the target's address space (via
// dyld_all_image_infos) and parses each one's export trie out of device memory.
// This is what makes the mapper work on AWS Device Farm, where the Mac-only
// HostFilesResolver's Xcode symbol files do not exist.
type ExportTrieResolver struct {
	c         *RSP
	nameDylib map[string]string // symbol name -> dylib path suffix that exports it
	loadAddr  map[string]uint64 // dylib path suffix -> in-memory load address (cache)
}

// NewExportTrieResolver maps the bootstrap helpers to the dylibs that export them.
func NewExportTrieResolver(c *RSP) *ExportTrieResolver {
	return &ExportTrieResolver{
		c: c,
		nameDylib: map[string]string{
			"mmap":               "/usr/lib/system/libsystem_kernel.dylib",
			"mprotect":           "/usr/lib/system/libsystem_kernel.dylib",
			"socket":             "/usr/lib/system/libsystem_kernel.dylib",
			"bind":               "/usr/lib/system/libsystem_kernel.dylib",
			"listen":             "/usr/lib/system/libsystem_kernel.dylib",
			"__error":            "/usr/lib/system/libsystem_kernel.dylib",
			"dlsym":              "/usr/lib/system/libdyld.dylib",
			"sel_registerName":   "/usr/lib/libobjc.A.dylib",
			"objc_readClassPair": "/usr/lib/libobjc.A.dylib",
			"objc_getClass":      "/usr/lib/libobjc.A.dylib",
			"objc_msgSend":       "/usr/lib/libobjc.A.dylib",
			"object_getClass":    "/usr/lib/libobjc.A.dylib",
			"class_addMethod":    "/usr/lib/libobjc.A.dylib",
		},
		loadAddr: map[string]uint64{},
	}
}

// Resolve groups names by exporting dylib, locates each dylib once, and parses
// its export trie for the requested symbols.
func (r *ExportTrieResolver) Resolve(names []string) (map[string]uint64, error) {
	byDylib := map[string][]string{}
	for _, n := range names {
		d, ok := r.nameDylib[n]
		if !ok {
			return nil, fmt.Errorf("export-trie resolver has no dylib mapping for %q", n)
		}
		byDylib[d] = append(byDylib[d], n)
	}
	// locate every needed dylib in one scan of the image list
	var need []string
	for d := range byDylib {
		if _, ok := r.loadAddr[d]; !ok {
			need = append(need, d)
		}
	}
	if len(need) > 0 {
		if err := r.locate(need); err != nil {
			return nil, err
		}
	}

	out := map[string]uint64{}
	for d, want := range byDylib {
		base, ok := r.loadAddr[d]
		if !ok || base == 0 {
			return nil, fmt.Errorf("dylib %s not found in loaded images", d)
		}
		trie, err := r.readExportTrie(base)
		if err != nil {
			return nil, fmt.Errorf("%s export trie: %w", d, err)
		}
		for _, n := range want {
			off, found, err := lookupExportTrie(trie, "_"+n)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n, err)
			}
			if !found {
				return nil, fmt.Errorf("%q not exported by %s", n, d)
			}
			out[n] = base + off
		}
	}
	return out, nil
}

// locate scans dyld_all_image_infos.infoArray for each wanted dylib path suffix
// and records its in-memory load address. Stops once all are found.
func (r *ExportTrieResolver) locate(suffixes []string) error {
	resp, err := r.c.Request("qShlibInfoAddr")
	if err != nil {
		return err
	}
	var aiaddr uint64
	if _, err := fmt.Sscanf(resp, "%x", &aiaddr); err != nil {
		return fmt.Errorf("qShlibInfoAddr %q: %w", resp, err)
	}
	hdr, err := r.c.Memory(aiaddr, 16)
	if err != nil {
		return err
	}
	count := binary.LittleEndian.Uint32(hdr[4:])
	arrayPtr := binary.LittleEndian.Uint64(hdr[8:])
	if arrayPtr == 0 || count == 0 || count > 100000 {
		return fmt.Errorf("implausible image info array: count=%d ptr=%#x", count, arrayPtr)
	}
	remaining := map[string]bool{}
	for _, s := range suffixes {
		remaining[s] = true
	}
	for i := uint32(0); i < count && len(remaining) > 0; i++ {
		ent, err := r.c.Memory(arrayPtr+uint64(i)*24, 16)
		if err != nil {
			return err
		}
		loadAddr := binary.LittleEndian.Uint64(ent[0:])
		pathPtr := binary.LittleEndian.Uint64(ent[8:])
		if pathPtr == 0 {
			continue
		}
		path := r.readCString(pathPtr, 256)
		for s := range remaining {
			if strings.HasSuffix(path, s) {
				r.loadAddr[s] = loadAddr
				delete(remaining, s)
			}
		}
	}
	if len(remaining) > 0 {
		var miss []string
		for s := range remaining {
			miss = append(miss, s)
		}
		return fmt.Errorf("dylibs not found in image list: %v", miss)
	}
	return nil
}

// readExportTrie reads a dylib's export trie from device memory by converting the
// LC_DYLD_EXPORTS_TRIE / LC_DYLD_INFO file offset into a mapped address using the
// image's own segment table (handles the shared cache's non-contiguous layout).
func (r *ExportTrieResolver) readExportTrie(loadAddr uint64) ([]byte, error) {
	head, err := r.c.Memory(loadAddr, 32)
	if err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint32(head) != machoMagic64 {
		return nil, fmt.Errorf("no Mach-O header at %#x", loadAddr)
	}
	ncmds := binary.LittleEndian.Uint32(head[16:])
	sizeofcmds := binary.LittleEndian.Uint32(head[20:])
	cmds, err := r.c.Memory(loadAddr+32, uint64(sizeofcmds))
	if err != nil {
		return nil, err
	}

	const (
		lcDyldInfo        = 0x2
		lcDyldInfoOnly    = 0x22
		lcDyldExportsTrie = 0x80000033
	)
	var textVM, leVM, leFileOff uint64
	var haveText, haveLE, haveTrie bool
	var exportOff, exportSize uint64

	off := 0
	for i := uint32(0); i < ncmds; i++ {
		if off+8 > len(cmds) {
			break
		}
		cmd := binary.LittleEndian.Uint32(cmds[off:])
		sz := binary.LittleEndian.Uint32(cmds[off+4:])
		if sz == 0 {
			break
		}
		switch cmd {
		case lcSegment64:
			name := cstr16(cmds[off+8 : off+24])
			vmaddr := binary.LittleEndian.Uint64(cmds[off+24:])
			fileoff := binary.LittleEndian.Uint64(cmds[off+40:])
			if trieDebug {
				vmsize := binary.LittleEndian.Uint64(cmds[off+32:])
				filesize := binary.LittleEndian.Uint64(cmds[off+48:])
				fmt.Fprintf(os.Stderr, "[seg] %-12s vmaddr=%#x vmsize=%#x fileoff=%#x filesize=%#x\n",
					name, vmaddr, vmsize, fileoff, filesize)
			}
			switch name {
			case "__TEXT":
				textVM, haveText = vmaddr, true
			case "__LINKEDIT":
				leVM, leFileOff, haveLE = vmaddr, fileoff, true
			}
		case lcDyldExportsTrie:
			// LC_DYLD_EXPORTS_TRIE is authoritative; a legacy LC_DYLD_INFO may also be
			// present with a stale/tiny export_off, so this always wins.
			exportOff = uint64(binary.LittleEndian.Uint32(cmds[off+8:]))
			exportSize = uint64(binary.LittleEndian.Uint32(cmds[off+12:]))
			haveTrie = true
			if trieDebug {
				fmt.Fprintf(os.Stderr, "[lc] LC_DYLD_EXPORTS_TRIE dataoff=%#x datasize=%#x\n", exportOff, exportSize)
			}
		case lcDyldInfo, lcDyldInfoOnly:
			if !haveTrie {
				exportOff = uint64(binary.LittleEndian.Uint32(cmds[off+40:]))
				exportSize = uint64(binary.LittleEndian.Uint32(cmds[off+44:]))
			}
			if trieDebug {
				fmt.Fprintf(os.Stderr, "[lc] LC_DYLD_INFO export_off=%#x export_size=%#x\n",
					binary.LittleEndian.Uint32(cmds[off+40:]), binary.LittleEndian.Uint32(cmds[off+44:]))
			}
		}
		off += int(sz)
	}
	if !haveText || !haveLE {
		return nil, fmt.Errorf("missing __TEXT/__LINKEDIT in image at %#x", loadAddr)
	}
	if exportOff == 0 || exportSize == 0 {
		return nil, fmt.Errorf("image at %#x has no export trie", loadAddr)
	}
	slide := loadAddr - textVM
	trieAddr := leVM + slide + (exportOff - leFileOff)
	if trieDebug {
		fmt.Fprintf(os.Stderr, "[trie] load=%#x textVM=%#x leVM=%#x leFileOff=%#x exportOff=%#x size=%#x slide=%#x trieAddr=%#x\n",
			loadAddr, textVM, leVM, leFileOff, exportOff, exportSize, slide, trieAddr)
	}
	trie, err := r.c.Memory(trieAddr, exportSize)
	if err != nil {
		return nil, err
	}
	if trieDebug {
		n := len(trie)
		if n > 48 {
			n = 48
		}
		fmt.Fprintf(os.Stderr, "[trie] first bytes: % x\n", trie[:n])
	}
	return trie, nil
}

func (r *ExportTrieResolver) readCString(addr uint64, max uint64) string {
	data, err := r.c.Memory(addr, max)
	if err != nil {
		return ""
	}
	if i := indexByte(data, 0); i >= 0 {
		return string(data[:i])
	}
	return string(data)
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// lookupExportTrie walks the dyld export trie for symbol and returns its address
// offset (from the image base) if found. Handles regular and absolute exports;
// errors on reexports (not expected for our bootstrap symbols).
func lookupExportTrie(trie []byte, symbol string) (uint64, bool, error) {
	const (
		flagsKindMask   = 0x03
		flagAbsolute    = 0x02
		flagReexport    = 0x08
		flagStubAndReso = 0x10
	)
	pos := 0
	rest := symbol
	for {
		if pos >= len(trie) {
			return 0, false, nil
		}
		termSize, n := uleb(trie, pos)
		pos += n
		if rest == "" {
			if termSize == 0 {
				return 0, false, nil
			}
			flags, n2 := uleb(trie, pos)
			p := pos + n2
			if flags&flagReexport != 0 || flags&flagStubAndReso != 0 {
				return 0, false, fmt.Errorf("unsupported export kind (flags=%#x)", flags)
			}
			addr, _ := uleb(trie, p)
			_ = flags & flagsKindMask
			_ = flagAbsolute
			return addr, true, nil
		}
		// skip this node's terminal payload to reach the child list
		childPos := pos + int(termSize)
		if childPos >= len(trie) {
			return 0, false, nil
		}
		childCount := int(trie[childPos])
		cp := childPos + 1
		next := -1
		for i := 0; i < childCount; i++ {
			edgeStart := cp
			for cp < len(trie) && trie[cp] != 0 {
				cp++
			}
			edge := string(trie[edgeStart:cp])
			cp++ // skip NUL
			childOff, m := uleb(trie, cp)
			cp += m
			if next == -1 && strings.HasPrefix(rest, edge) {
				rest = rest[len(edge):]
				next = int(childOff)
			}
		}
		if next == -1 {
			return 0, false, nil
		}
		pos = next
	}
}

// uleb reads an unsigned LEB128 at data[pos], returning the value and bytes read.
func uleb(data []byte, pos int) (uint64, int) {
	var result uint64
	var shift uint
	i := 0
	for pos+i < len(data) {
		b := data[pos+i]
		result |= uint64(b&0x7f) << shift
		i++
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return result, i
}
