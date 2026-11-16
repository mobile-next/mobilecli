package machomap

import (
	"encoding/binary"
	"fmt"
)

// categoryHelpers are the extra runtime functions needed only when the image has
// categories (resolved lazily so category-free dylibs don't require them).
var categoryHelpers = []string{"sel_registerName", "object_getClass", "class_addMethod"}

// applyCategories registers each __objc_catlist category onto its target class.
// objc_readClassPair handles classes but not categories, and there is no public
// API to attach a category from a mapped image, so we read each category_t and
// its (small/relative) method lists out of the just-written image and add every
// method with class_addMethod — instance methods on the class, class methods on
// its metaclass. The target class is a bound framework class (e.g. NSURLSession),
// already realized, so adding methods is safe. Must run before the initializer,
// which may method-swizzle the methods the categories add.
func applyCategories(c *RSP, t *Target, base uint64, m *Macho, h map[string]uint64) (int, error) {
	sec, ok := m.Sections["__objc_catlist"]
	if !ok || sec.Size == 0 {
		return 0, nil
	}
	selReg, getClass, addMethod := h["sel_registerName"], h["object_getClass"], h["class_addMethod"]
	if selReg == 0 || getClass == 0 || addMethod == 0 {
		return 0, fmt.Errorf("category helpers unresolved")
	}

	applied := 0
	count := int(sec.Size / 8) // #nosec G115 -- a section size in a small dylib
	for i := 0; i < count; i++ {
		// the catlist slot holds base+category_vmaddr after the image write
		catAddr, err := c.readPtr(base + sec.Addr + uint64(i)*8)
		if err != nil {
			return applied, err
		}
		n, err := applyCategory(c, t, catAddr, getClass, selReg, addMethod)
		applied += n
		if err != nil {
			return applied, err
		}
	}
	return applied, nil
}

// applyCategory adds one category_t's instance and class methods to its class.
// category_t layout: name(+0) cls(+8) instanceMethods(+16) classMethods(+24).
func applyCategory(c *RSP, t *Target, catAddr, getClass, selReg, addMethod uint64) (int, error) {
	clsAddr, err := c.readPtr(catAddr + 8)
	if err != nil || clsAddr == 0 {
		return 0, err
	}
	applied := 0
	for _, ml := range []struct {
		fieldOff uint64
		isClass  bool
	}{{16, false}, {24, true}} {
		listAddr, err := c.readPtr(catAddr + ml.fieldOff)
		if err != nil {
			return applied, err
		}
		if listAddr == 0 {
			continue
		}
		target := clsAddr
		if ml.isClass {
			if target, err = t.Call(getClass, clsAddr); err != nil { // metaclass
				return applied, err
			}
		}
		n, err := applyMethodList(c, t, listAddr, target, selReg, addMethod)
		if err != nil {
			return applied, err
		}
		applied += n
	}
	return applied, nil
}

// applyMethodList adds every method in a small (relative) method_list_t at
// listAddr to target.
func applyMethodList(c *RSP, t *Target, listAddr, target, selReg, addMethod uint64) (int, error) {
	hdr, err := c.Memory(listAddr, 8)
	if err != nil {
		return 0, err
	}
	eaf := binary.LittleEndian.Uint32(hdr)
	count := binary.LittleEndian.Uint32(hdr[4:])
	entsize := uint64(eaf & 0xFFFC)
	if eaf&0x80000000 == 0 {
		return 0, fmt.Errorf("non-relative method list (entsizeAndFlags=%#x) not supported", eaf)
	}
	applied := 0
	for k := uint32(0); k < count; k++ {
		entry := listAddr + 8 + uint64(k)*entsize
		rel, err := c.Memory(entry, 12) // {int32 name, int32 types, int32 imp}, each self-relative
		if err != nil {
			return applied, err
		}
		nameRel := int32(binary.LittleEndian.Uint32(rel))      // #nosec G115 -- relative offsets are signed 32-bit values
		typesRel := int32(binary.LittleEndian.Uint32(rel[4:])) // #nosec G115 -- relative offsets are signed 32-bit values
		impRel := int32(binary.LittleEndian.Uint32(rel[8:]))   // #nosec G115 -- relative offsets are signed 32-bit values
		nameSlot := relAddr(entry+0, nameRel)                  // -> selref slot (holds name cstring / SEL)
		typesAddr := relAddr(entry+4, typesRel)                // -> type-encoding cstring
		impAddr := relAddr(entry+8, impRel)                    // -> IMP in our mapped __TEXT
		namePtr, err := c.readPtr(nameSlot)
		if err != nil {
			return applied, err
		}
		sel, err := t.Call(selReg, namePtr)
		if err != nil {
			return applied, err
		}
		if _, err := t.Call(addMethod, target, sel, impAddr, typesAddr); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}

// relAddr adds a signed self-relative offset to a field address.
func relAddr(fieldAddr uint64, rel int32) uint64 {
	return uint64(int64(fieldAddr) + int64(rel)) // #nosec G115 -- pointer arithmetic on a signed offset
}
