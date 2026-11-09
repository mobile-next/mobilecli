package machomap

import (
	"encoding/hex"
	"os"
	"testing"
)

func realAgentDylib(t *testing.T) []byte {
	t.Helper()
	path := os.Getenv("AGENT_DYLIB")
	if path == "" {
		path = "../../../agents/ios/agent-dev.dylib"
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Skipf("agent dylib not available at %s; build it or set AGENT_DYLIB", path)
	}
	return data
}

// TestParserReadsTheRealAgent checks that the Mach-O and fixup parsers find the
// parts the mapper needs in a real agent build. Set AGENT_DYLIB to override the
// path (default: the build output in agents/ios). The test skips if the dylib is
// missing or is the empty placeholder that lint jobs create.
func TestParserReadsTheRealAgent(t *testing.T) {
	data := realAgentDylib(t)

	m, err := ParseMacho(data)
	if err != nil {
		t.Fatalf("ParseMacho: %v", err)
	}
	image, imageSize, _ := m.BuildImage()
	if imageSize == 0 {
		t.Error("image size is 0")
	}
	if !m.HasChained {
		t.Fatal("expected chained fixups")
	}

	imports, _, segs, err := ParseChainedFixups(data, m.ChainedOff)
	if err != nil {
		t.Fatalf("ParseChainedFixups: %v", err)
	}
	if len(imports) == 0 {
		t.Error("found no imports")
	}
	// dummy import addresses just to exercise the walker and count rebases/binds
	rebases, binds, err := ApplyFixups(image, segs, 0x100000000, make([]uint64, len(imports)))
	if err != nil {
		t.Fatalf("ApplyFixups: %v", err)
	}
	if rebases == 0 {
		t.Error("found no rebases")
	}
	if binds == 0 {
		t.Error("found no binds")
	}

	if got := m.SectionCount("__objc_selrefs"); got == 0 {
		t.Error("found no selector references")
	}
	if got := m.SectionCount("__objc_classlist"); got == 0 {
		t.Error("found no classes")
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
