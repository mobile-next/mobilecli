package machomap

import "encoding/hex"

// In-process loop stubs (source: stubs_arm64.asm in this package — named .asm so
// the Go toolchain ignores it; identical to the prototype's stubs.s). Each
// replaces a per-element vCont round-trip with a
// single call that loops inside the target, collapsing ~286 remote calls to ~8
// and taking the map step from ~8s to ~0.4s:
//
//	rstub_dlsym (x0=dlsym, x1=count, x2=names[], x3=out[], x4=handle): out[i]=dlsym(handle,names[i])
//	rstub_selref(x0=sel_registerName, x1=count, x2=slots):            slots[i]=sel_registerName(slots[i])
//	rstub_class (x0=readClassPair, x1=count, x2=classlist, x3=imginfo): readClassPair(classlist[i],imginfo)
//
// Regenerate: clang -arch arm64 -c stubs_arm64.asm -o stubs.o && extract __TEXT,__text.
const stubHex = "" +
	"f353bca9f55b01a9f76302a9fd7b03a9f30300aaf40301aaf50302aaf60303aa" +
	"f70304aa180080d21f0314ebea000054e00317aaa17a78f860023fd6c07a38f8" +
	"18070091f9ffff17fd7b43a9f76342a9f55b41a9f353c4a8c0035fd6f353bca9" +
	"f55b01a9f76302a9fd7b03a9f30300aaf40301aaf50302aa180080d21f0314eb" +
	"ca000054a07a78f860023fd6a07a38f818070091faffff17fd7b43a9f76342a9" +
	"f55b41a9f353c4a8c0035fd6f353bca9f55b01a9f76302a9fd7b03a9f30300aa" +
	"f40301aaf50302aaf60303aa180080d21f0314ebca000054a07a78f8e10316aa" +
	"60023fd618070091faffff17fd7b43a9f76342a9f55b41a9f353c4a8c0035fd6"

// StubBytes is the assembled machine code for the three loop stubs.
var StubBytes = mustHex(stubHex)

// Offsets of each stub within StubBytes.
const (
	StubDlsymOff  = 0x00
	StubSelrefOff = 0x5c
	StubClassOff  = 0xac
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
