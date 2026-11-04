package machomap

import (
	"encoding/hex"
	"os"
	"testing"
)

// theGoParserReproducesThePythonPrototypeCounts loads the real network agent
// dylib and checks that the ported Mach-O/fixup logic yields exactly the numbers
// the proven Python prototype (inject_map.py) printed on-device:
//
//	image_size=0x1c000  rebases=483  binds=244  imports=97
//	selrefs=184  classes=5  categories=2  initializers=1
//
// Set AGENT_DYLIB to override the path (default /tmp/agent-dev.dylib). The test
// skips if the dylib is not present so CI without it stays green.
func TestGoParserMatchesPythonCounts(t *testing.T) {
	path := os.Getenv("AGENT_DYLIB")
	if path == "" {
		path = "/tmp/agent-dev.dylib"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("agent dylib not available (%v); set AGENT_DYLIB to run", err)
	}

	m, err := ParseMacho(data)
	if err != nil {
		t.Fatalf("ParseMacho: %v", err)
	}
	image, imageSize, _ := m.BuildImage()
	if imageSize != 0x1c000 {
		t.Errorf("image size = %#x, want 0x1c000", imageSize)
	}
	if !m.HasChained {
		t.Fatal("expected chained fixups")
	}

	imports, _, segs, err := ParseChainedFixups(data, m.ChainedOff)
	if err != nil {
		t.Fatalf("ParseChainedFixups: %v", err)
	}
	if len(imports) != 97 {
		t.Errorf("imports = %d, want 97", len(imports))
	}
	// dummy import addresses just to exercise the walker and count rebases/binds
	rebases, binds, err := ApplyFixups(image, segs, 0x100000000, make([]uint64, len(imports)))
	if err != nil {
		t.Fatalf("ApplyFixups: %v", err)
	}
	if rebases != 483 {
		t.Errorf("rebases = %d, want 483", rebases)
	}
	if binds != 244 {
		t.Errorf("binds = %d, want 244", binds)
	}

	if got := m.SectionCount("__objc_selrefs"); got != 184 {
		t.Errorf("selrefs = %d, want 184", got)
	}
	if got := m.SectionCount("__objc_classlist"); got != 5 {
		t.Errorf("classes = %d, want 5", got)
	}
	if got := m.SectionCount("__objc_catlist"); got != 2 {
		t.Errorf("categories = %d, want 2", got)
	}
	if len(m.InitSections) != 1 || m.InitSections[0].Size != 4 {
		t.Errorf("init sections = %+v, want one of size 4", m.InitSections)
	}
}

// TestStubBytesDecode guards the embedded stub machine code: length and the
// first instruction (stp x19,x20,[sp,#-64]! = 0xa9bc53f3) must be intact.
func TestStubBytesDecode(t *testing.T) {
	if len(StubBytes) != 256 {
		t.Fatalf("stub bytes = %d, want 256", len(StubBytes))
	}
	first := hex.EncodeToString(StubBytes[:4])
	if first != "f353bca9" {
		t.Errorf("first stub word = %s, want f353bca9", first)
	}
}
