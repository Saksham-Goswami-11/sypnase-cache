package hnsw

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
)

func TestSnapshotRoundtrip(t *testing.T) {
	idx := NewIndex(16, 200)

	// Insert 100 vectors
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		vec := randomVector(rng, 64)
		meta := map[string]string{
			"id":   string(rune(i)),
			"test": "value",
		}
		_ = idx.Insert(string(rune(i)), vec, meta)
	}

	// Create snapshot
	var buf bytes.Buffer
	err := idx.Snapshot(&buf)
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	// Load snapshot
	loadedIdx, err := LoadSnapshot(&buf)
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}

	// Verify header
	if loadedIdx.M != idx.M || loadedIdx.Mmax0 != idx.Mmax0 ||
		loadedIdx.EfConstruction != idx.EfConstruction || loadedIdx.Ef != idx.Ef ||
		loadedIdx.currentMaxLayer != idx.currentMaxLayer || loadedIdx.entryPoint != idx.entryPoint {
		t.Fatalf("header mismatch after load")
	}

	// Verify nodes count
	if loadedIdx.Len() != idx.Len() {
		t.Fatalf("expected %d nodes, got %d", idx.Len(), loadedIdx.Len())
	}

	// Verify node details
	for i, node := range idx.nodes {
		loadedNode := loadedIdx.nodes[i]
		if loadedNode.ID != node.ID {
			t.Fatalf("node %d ID mismatch: %d != %d", i, node.ID, loadedNode.ID)
		}
		if !reflect.DeepEqual(loadedNode.Vector, node.Vector) {
			t.Fatalf("node %d vector mismatch", i)
		}
		if !reflect.DeepEqual(loadedNode.Metadata, node.Metadata) {
			t.Fatalf("node %d metadata mismatch: %v != %v", i, node.Metadata, loadedNode.Metadata)
		}
		for l, links := range node.Links {
			loadedLinks := loadedNode.Links[l]
			if !reflect.DeepEqual(links, loadedLinks) {
				if len(links) != 0 || len(loadedLinks) != 0 {
					t.Fatalf("node %d layer %d links mismatch: %v != %v", i, l, links, loadedLinks)
				}
			}
		}
	}

	// Verify reverse mapping
	if !reflect.DeepEqual(idx.reverseMap, loadedIdx.reverseMap) {
		t.Fatalf("reverseMap mismatch")
	}
	if !reflect.DeepEqual(idx.idMap, loadedIdx.idMap) {
		t.Fatalf("idMap mismatch")
	}

	// Query both indexes
	q := randomVector(rng, 64)
	res1, _ := idx.Search(q, 5, 0)
	res2, _ := loadedIdx.Search(q, 5, 0)

	if !reflect.DeepEqual(res1, res2) {
		t.Fatalf("search results mismatch after load")
	}
}

func TestSnapshotInvalidChecksum(t *testing.T) {
	idx := NewIndex(16, 200)
	_ = idx.Insert("key", []float32{1.0}, nil)

	var buf bytes.Buffer
	_ = idx.Snapshot(&buf)

	data := buf.Bytes()
	// Corrupt checksum
	data[len(data)-1] ^= 0xFF

	_, err := LoadSnapshot(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error on corrupt checksum")
	}
}
