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

	scratchSize  = 0x10000 // rw scratch for staged strings/tables
	stubPageSize = 0x4000  // r-x page holding the loop stubs + landing pad
	// landingPadOffset is the call-return breakpoint address within the stub
	// page, placed well past the stub bytes so callee code never executes it.
	landingPadOffset  = 0x800
	defaultWriteChunk = 1024
)

// bootstrapNames are the runtime helpers the mapper itself needs; the Resolver
// supplies their in-target addresses without any remote round-trips.
var bootstrapNames = []string{"mmap", "mprotect", "dlsym", "sel_registerName", "objc_readClassPair"}

// Resolver resolves symbol names to their runtime addresses in the target process.
type Resolver interface {
	Resolve(names []string) (map[string]uint64, error)
}

// Options controls the map.
type Options struct {
	RunInit    bool // run the dylib's offset-based initializers (starts the agent)
	WriteChunk int  // M-packet chunk size; 0 => defaultWriteChunk
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
	Categories int
	Unresolved []string
}

// Map maps dylib into the already-attached process behind c and executes its
// initializers (unless Options.RunInit is false). The process must be attached
// (QSetDetachOnError + vAttach) by the caller; Map borrows a thread and restores
// it before returning.
func Map(c *RSP, dylib []byte, resolver Resolver, opt Options) (*Result, error) {
	m, err := ParseMacho(dylib)
	if err != nil {
		return nil, err
	}
	image, imageSize, mapped := m.BuildImage()

	helpers, err := resolver.Resolve(bootstrapNames)
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

	chunk := opt.WriteChunk
	if chunk == 0 {
		chunk = defaultWriteChunk
	}
	s := &mapSession{
		c: c, t: t, m: m, dylib: dylib, image: image, chunk: chunk,
		helpers: helpers, resolver: resolver,
		result: &Result{ImageSize: imageSize},
	}

	steps := []func() error{
		func() error { return s.reserveImage(imageSize) },
		func() error { return s.remapWritableSegments(mapped) },
		s.setupScratchAndStubs,
		s.bindImports,
		s.writeImage,
		s.registerClasses,
		s.applyImageCategories,
		func() error {
			if opt.RunInit {
				return s.runInitializers()
			}
			return nil
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return s.result, nil
}

// mapSession holds the state shared across the steps of a single Map call.
type mapSession struct {
	c        *RSP
	t        *Target
	m        *Macho
	dylib    []byte
	image    []byte
	base     uint64
	chunk    int
	scratch  uint64 // next free offset in the rw scratch region
	stub     uint64 // base of the r-x stub page
	helpers  map[string]uint64
	resolver Resolver
	result   *Result
}

// alloc hands out 8-byte-aligned chunks of the rw scratch region.
func (s *mapSession) alloc(size uint64) uint64 {
	s.scratch = (s.scratch + 7) &^ 7
	p := s.scratch
	s.scratch += size
	return p
}

// reserveImage mmaps the whole image r-x (executable). iOS fixes max_protection
// at mmap time, so EXEC must be requested now; the writable segments are then
// remapped rw over this reservation.
func (s *mapSession) reserveImage(imageSize uint64) error {
	base, err := s.t.Call(s.helpers["mmap"], 0, imageSize, protRead|protExec, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return err
	}
	if base == neg1 {
		return fmt.Errorf("reserve mmap failed (MAP_FAILED)")
	}
	s.base = base
	s.result.Base = base
	return nil
}

// remapWritableSegments MAP_FIXED-remaps each writable, non-__TEXT segment rw.
func (s *mapSession) remapWritableSegments(mapped []Segment) error {
	for _, seg := range mapped {
		if seg.Name == "__TEXT" || seg.InitProt&vmProtWrite == 0 {
			continue
		}
		at := s.base + seg.VMAddr
		got, err := s.t.Call(s.helpers["mmap"], at, seg.VMSize, protRead|protWrite, mapAnon|mapPrivate|mapFixed, neg1, 0)
		if err != nil {
			return err
		}
		if got != at {
			return fmt.Errorf("MAP_FIXED %s @ %#x -> %#x", seg.Name, at, got)
		}
	}
	return nil
}

// setupScratchAndStubs maps the rw scratch region and the r-x stub page, writes
// the loop stubs, and points the call-return breakpoint at a private address in
// the stub page (rather than the thread's live PC) so re-entrant callee code —
// NSLog/libdispatch in the initializer — can't trip it early.
func (s *mapSession) setupScratchAndStubs() error {
	scratch, err := s.t.Call(s.helpers["mmap"], 0, scratchSize, protRead|protWrite, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return err
	}
	stub, err := s.t.Call(s.helpers["mmap"], 0, stubPageSize, protRead|protExec, mapAnon|mapPrivate, neg1, 0)
	if err != nil {
		return err
	}
	if err := s.c.WriteMem(stub, StubBytes, s.chunk); err != nil {
		return err
	}
	s.scratch, s.stub = scratch, stub
	return s.t.SetLanding(stub + landingPadOffset)
}

// bindImports resolves every imported symbol in one in-process rstub_dlsym call
// (names + a pointer table + an output table staged in scratch), then applies
// the chained fixups to the image buffer locally.
func (s *mapSession) bindImports() error {
	if !s.m.HasChained {
		return nil
	}
	imports, weak, segs, err := ParseChainedFixups(s.dylib, s.m.ChainedOff)
	if err != nil {
		return err
	}
	s.result.Imports = len(imports)

	var blob []byte
	offs := make([]int, len(imports))
	for i, n := range imports {
		offs[i] = len(blob)
		blob = append(blob, stripOneUnderscore(n)...)
		blob = append(blob, 0)
	}
	blobBase := s.alloc(uint64(len(blob)))
	if err := s.c.WriteMem(blobBase, blob, s.chunk); err != nil {
		return err
	}

	namesTbl := s.alloc(uint64(len(imports)) * 8)
	tbl := make([]byte, len(imports)*8)
	for i := range imports {
		binary.LittleEndian.PutUint64(tbl[i*8:], blobBase+uint64(offs[i])) // #nosec G115 -- offsets into our own blob are never negative
	}
	if err := s.c.WriteMem(namesTbl, tbl, s.chunk); err != nil {
		return err
	}
	outTbl := s.alloc(uint64(len(imports)) * 8)
	if _, err := s.t.Call(s.stub+StubDlsymOff, s.helpers["dlsym"], uint64(len(imports)), namesTbl, outTbl, rtldDefault); err != nil {
		return err
	}

	raw, err := s.c.Memory(outTbl, uint64(len(imports))*8)
	if err != nil {
		return err
	}
	importAddrs := make([]uint64, len(imports))
	for i := range imports {
		importAddrs[i] = binary.LittleEndian.Uint64(raw[i*8:])
		// A weak import may legitimately resolve to 0 (the code null-checks it);
		// a missing non-weak import would make the initializer call a null pointer
		// and crash the app, so fail now, before the image is written.
		if importAddrs[i] == 0 && !weak[i] {
			s.result.Unresolved = append(s.result.Unresolved, imports[i])
		}
	}
	if len(s.result.Unresolved) > 0 {
		return fmt.Errorf("unresolved imports (missing symbols in target): %v", s.result.Unresolved)
	}
	rebases, binds, err := ApplyFixups(s.image, segs, s.base, importAddrs)
	if err != nil {
		return err
	}
	s.result.Rebases, s.result.Binds = rebases, binds
	return nil
}

// writeImage writes the fixed-up image into the reserved mapping.
func (s *mapSession) writeImage() error {
	return s.c.WriteMem(s.base, s.image, s.chunk)
}

// registerClasses uniques the image's selrefs and registers its classes with
// objc_readClassPair — one in-process stub call each.
func (s *mapSession) registerClasses() error {
	classlist, ok := s.m.Sections["__objc_classlist"]
	if !ok || classlist.Size == 0 {
		return nil
	}
	if selrefs, ok := s.m.Sections["__objc_selrefs"]; ok && selrefs.Size > 0 {
		n := selrefs.Size / 8
		if _, err := s.t.Call(s.stub+StubSelrefOff, s.helpers["sel_registerName"], n, s.base+selrefs.Addr); err != nil {
			return err
		}
		s.result.Selrefs = int(n) // #nosec G115 -- a count from a small dylib
	}
	var imageinfo uint64
	if ii, ok := s.m.Sections["__objc_imageinfo"]; ok {
		imageinfo = s.base + ii.Addr
	}
	n := classlist.Size / 8
	if _, err := s.t.Call(s.stub+StubClassOff, s.helpers["objc_readClassPair"], n, s.base+classlist.Addr, imageinfo); err != nil {
		return err
	}
	s.result.Classes = int(n) // #nosec G115 -- a count from a small dylib
	return nil
}

// applyImageCategories attaches the image's __objc_catlist categories (which
// objc_readClassPair does not), before the initializer that may swizzle them.
func (s *mapSession) applyImageCategories() error {
	if s.m.SectionCount("__objc_catlist") == 0 {
		return nil
	}
	helpers, err := s.resolver.Resolve(categoryHelpers)
	if err != nil {
		return fmt.Errorf("resolve category helpers: %w", err)
	}
	applied, err := applyCategories(s.c, s.t, s.base, s.m, helpers)
	if err != nil {
		return err
	}
	s.result.Categories = applied
	return nil
}

// runInitializers runs each offset-based initializer (the agent's C constructor),
// which is what starts the agent.
func (s *mapSession) runInitializers() error {
	for _, is := range s.m.InitSections {
		for o := uint64(0); o < is.Size; o += 4 {
			initOff := uint64(binary.LittleEndian.Uint32(s.image[is.Addr+o:]))
			if _, err := s.t.Call(s.base + initOff); err != nil {
				return err
			}
		}
	}
	return nil
}

// stripOneUnderscore removes exactly one leading linker underscore (not all).
func stripOneUnderscore(s string) string {
	if len(s) > 0 && s[0] == '_' {
		return s[1:]
	}
	return s
}
