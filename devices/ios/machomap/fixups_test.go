package machomap

import (
	"encoding/binary"
	"testing"
)

func chainedFixupsHeader(importsFormat, symbolsFormat uint32) []byte {
	header := make([]byte, 28)
	binary.LittleEndian.PutUint32(header[20:], importsFormat)
	binary.LittleEndian.PutUint32(header[24:], symbolsFormat)
	return header
}

func TestParserAcceptsThePlainFixupFormats(t *testing.T) {
	header := chainedFixupsHeader(dyldChainedImport, dyldChainedSymbolsUncompressed)

	if err := requireSupportedFixupFormats(header); err != nil {
		t.Errorf("plain formats refused: %v", err)
	}
}

func TestParserRefusesImportsWithAddends(t *testing.T) {
	const importsWithAddend = 2
	header := chainedFixupsHeader(importsWithAddend, dyldChainedSymbolsUncompressed)

	if err := requireSupportedFixupFormats(header); err == nil {
		t.Error("imports with addends would be read with the wrong layout")
	}
}

func TestParserRefusesCompressedSymbols(t *testing.T) {
	const zlibCompressedSymbols = 1
	header := chainedFixupsHeader(dyldChainedImport, zlibCompressedSymbols)

	if err := requireSupportedFixupFormats(header); err == nil {
		t.Error("compressed symbol names would be read as plain text")
	}
}
