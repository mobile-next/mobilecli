package machomap

import (
	"encoding/binary"
	"fmt"
)

// mmap/mprotect protection and flag bits, and the W^X-driven mapping strategy:
// iOS fixes a mapping's max_protection at mmap time and forbids a page that is
// both writable and executable, so we reserve the whole image r-x (executable)
// and MAP_FIXED-remap the writable segments rw; the debugger writes the r-x code
// pages itself.
const (
	protRead    = 1
	protWrite   = 2
	protExec    = 4
	vmProtWrite = 2
	mapPrivate  = 0x0002
	mapAnon     = 0x1000
	mapFixed    = 0x0010
	neg1        = ^uint64(0) // -1 (mmap fd, and RTLD_DEFAULT base below)
	rtldDefault = ^uint64(0) - 1
)

// bootstrapNames are the runtime helpers the mapper itself needs; the Resolver
// supplies their in-target addresses without any remote round-trips.
var bootstrapNames = []string{"mmap", "mprotect", "dlsym", "sel_registerName", "objc_readClassPair"}

// Resolver resolves symbol names to their runtime addresses in the target
// process. Two implementations exist: host Xcode symbol files + shared-cache
// slide (fast, Mac-only — for parity testing), and an in-memory export-trie
// reader (host-independent — for AWS Device Farm).
type Resolver interface {
	Resolve(names []string) (map[string]uint64, error)
}

// Options controls the map.
type Options struct {
	RunInit    bool // run the dylib's offset-based initializers (starts the agent)
	WriteChunk int  // M-packet chunk size; 0 => 1024
}

// Result reports what was mapped.
type Result struct {
	Base       uint64
	ImageSize  uint64
	Rebases    int
	Binds      int
	Imports    int
	Selrefs    int
	Classes    int
	Unresolved []string
}

// Map maps dylib into the already-attached process behind c and executes its
// initializers (unless Options.RunInit is false). The process must be attached
// (QSetDetachOnError + vAttach) by the caller; Map borrows a thread and restores
// it before returning.
func Map(c *RSP, dylib []byte, res Resolver, opt Options) (*Result, error) {
	m, err := ParseMacho(dylib)
	if err != nil {
		return nil, err
	}
	image, imageSize, mapped := m.BuildImage()
	chunk := opt.WriteChunk
	if chunk == 0 {
		chunk = 1024
	}

	helpers, err := res.Resolve(bootstrapNames)
	if err != nil {
		return nil, fmt.Errorf("resolve bootstrap helpers: %w", err)
	}
	for _, n := range bootstrapNames {
		if helpers[n] == 0 {
			return nil, fmt.Errorf("bootstrap helper %q unresolved", n)
		}
	}

	t, err := NewTarget(c)
	if err != nil {
		return nil, err
	}
	defer func() { _ = t.Restore() }()

	mmap := helpers["mmap"]
	res0 := &Result{ImageSize: imageSize}

	// 1. reserve the image r-x
	base, err := t.Call(mmap, 0, imageSize, protRead|protExec, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return nil, err
	}
	if base == neg1 {
		return nil, fmt.Errorf("reserve mmap failed (MAP_FAILED)")
	}
	res0.Base = base

	// 2. MAP_FIXED-remap writable segments rw
	for _, s := range mapped {
		if s.Name != "__TEXT" && s.InitProt&vmProtWrite != 0 {
			got, err := t.Call(mmap, base+s.VMAddr, s.VMSize, protRead|protWrite,
				mapAnon|mapPrivate|mapFixed, neg1, 0)
			if err != nil {
				return nil, err
			}
			if got != base+s.VMAddr {
				return nil, fmt.Errorf("MAP_FIXED %s @ %#x -> %#x", s.Name, base+s.VMAddr, got)
			}
		}
	}

	// 3. RW scratch for staged tables, and an r-x page for the loop stubs
	scratch, err := t.Call(mmap, 0, 0x10000, protRead|protWrite, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return nil, err
	}
	stubMem, err := t.Call(mmap, 0, 0x4000, protRead|protExec, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return nil, err
	}
	if err := c.WriteMem(stubMem, StubBytes, chunk); err != nil {
		return nil, err
	}
	cur := scratch
	alloc := func(size uint64) uint64 {
		cur = (cur + 7) &^ 7
		p := cur
		cur += size
		return p
	}

	// 4. resolve all imports in ONE call via rstub_dlsym, then apply fixups locally
	if m.HasChained {
		imports, segs, err := ParseChainedFixups(dylib, m.ChainedOff)
		if err != nil {
			return nil, err
		}
		res0.Imports = len(imports)
		// stage import name strings (one write) + a pointer table + an output table
		var blob []byte
		offs := make([]int, len(imports))
		for i, n := range imports {
			offs[i] = len(blob)
			blob = append(blob, stripOneUnderscore(n)...)
			blob = append(blob, 0)
		}
		blobBase := alloc(uint64(len(blob)))
		if err := c.WriteMem(blobBase, blob, chunk); err != nil {
			return nil, err
		}
		namesTbl := alloc(uint64(len(imports)) * 8)
		tbl := make([]byte, len(imports)*8)
		for i := range imports {
			binary.LittleEndian.PutUint64(tbl[i*8:], blobBase+uint64(offs[i]))
		}
		if err := c.WriteMem(namesTbl, tbl, chunk); err != nil {
			return nil, err
		}
		outTbl := alloc(uint64(len(imports)) * 8)
		if _, err := t.Call(stubMem+StubDlsymOff, helpers["dlsym"], uint64(len(imports)),
			namesTbl, outTbl, rtldDefault); err != nil {
			return nil, err
		}
		raw, err := c.Memory(outTbl, uint64(len(imports))*8)
		if err != nil {
			return nil, err
		}
		importAddrs := make([]uint64, len(imports))
		for i := range imports {
			importAddrs[i] = binary.LittleEndian.Uint64(raw[i*8:])
			if importAddrs[i] == 0 {
				res0.Unresolved = append(res0.Unresolved, imports[i])
			}
		}
		rb, bn, err := ApplyFixups(image, segs, base, importAddrs)
		if err != nil {
			return nil, err
		}
		res0.Rebases, res0.Binds = rb, bn
	}

	// 5. write the fixed-up image
	if err := c.WriteMem(base, image, chunk); err != nil {
		return nil, err
	}

	// 6. register ObjC metadata (selrefs then classes), one stub call each
	if n := m.SectionCount("__objc_classlist"); n > 0 {
		if sel := m.SectionCount("__objc_selrefs"); sel > 0 {
			sr := m.Sections["__objc_selrefs"]
			if _, err := t.Call(stubMem+StubSelrefOff, helpers["sel_registerName"], sel, base+sr.Addr); err != nil {
				return nil, err
			}
			res0.Selrefs = int(sel)
		}
		cl := m.Sections["__objc_classlist"]
		var imageinfo uint64
		if ii, ok := m.Sections["__objc_imageinfo"]; ok {
			imageinfo = base + ii.Addr
		}
		if _, err := t.Call(stubMem+StubClassOff, helpers["objc_readClassPair"], n, base+cl.Addr, imageinfo); err != nil {
			return nil, err
		}
		res0.Classes = int(n)
	}

	// 7. run initializers (starts the agent) unless suppressed
	if opt.RunInit {
		for _, is := range m.InitSections {
			for o := uint64(0); o < is.Size; o += 4 {
				initOff := uint64(binary.LittleEndian.Uint32(image[is.Addr+o:]))
				if _, err := t.Call(base + initOff); err != nil {
					return nil, err
				}
			}
		}
	}

	return res0, nil
}

// stripOneUnderscore removes exactly one leading linker underscore (not all).
func stripOneUnderscore(s string) string {
	if len(s) > 0 && s[0] == '_' {
		return s[1:]
	}
	return s
}
