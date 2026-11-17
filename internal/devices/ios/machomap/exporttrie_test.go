package machomap

import "testing"

func TestExportTrieLookupGivesUpOnAnEmptyEdge(t *testing.T) {
	nodeWithAnEmptyEdgeToItself := []byte{0, 1, 0, 0}

	_, found, err := lookupExportTrie(nodeWithAnEmptyEdgeToItself, "_symbol")

	if found || err != nil {
		t.Errorf("found = %v, err = %v; want not found", found, err)
	}
}

func TestExportTrieLookupGivesUpOnAHugeChildOffset(t *testing.T) {
	childOffsetThatOverflowsInt := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}
	trie := append([]byte{0, 1, '_', 0}, childOffsetThatOverflowsInt...)

	_, found, err := lookupExportTrie(trie, "_symbol")

	if found || err != nil {
		t.Errorf("found = %v, err = %v; want not found", found, err)
	}
}
