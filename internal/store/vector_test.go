package store

import (
	"math"
	"testing"
)

func TestVSetAndVGet(t *testing.T) {
	s := New()
	vec := []float32{0.1, 0.2, 0.3, 0.4}
	meta := map[string]string{"source": "test.pdf", "page": "1"}

	err := s.VSet("docs", "chunk:1", 4, vec, meta)
	if err != nil {
		t.Fatalf("VSet failed: %v", err)
	}

	entry, ok := s.VGet("docs", "chunk:1")
	if !ok {
		t.Fatal("VGet returned not found")
	}
	if entry.ID != "chunk:1" {
		t.Errorf("expected ID chunk:1, got %s", entry.ID)
	}
	for i, v := range entry.Vector {
		if math.Abs(float64(v-vec[i])) > 1e-6 {
			t.Errorf("vector[%d]: expected %f, got %f", i, vec[i], v)
		}
	}
	if entry.Metadata["source"] != "test.pdf" {
		t.Errorf("metadata source: expected test.pdf, got %s", entry.Metadata["source"])
	}
}

func TestVSetDimensionMismatch(t *testing.T) {
	s := New()
	vec := []float32{0.1, 0.2, 0.3, 0.4}

	err := s.VSet("docs", "chunk:1", 3, vec, nil) // declared 3 but gave 4
	if err == nil {
		t.Fatal("expected dimension mismatch error, got nil")
	}
}

func TestVDel(t *testing.T) {
	s := New()
	s.VSet("docs", "chunk:1", 2, []float32{0.1, 0.2}, nil)

	if !s.VDel("docs", "chunk:1") {
		t.Error("VDel should return true for existing entry")
	}
	if s.VDel("docs", "chunk:1") {
		t.Error("VDel should return false for already-deleted entry")
	}
	if s.VDel("docs", "nonexistent") {
		t.Error("VDel should return false for nonexistent entry")
	}
	if s.VDel("no-namespace", "chunk:1") {
		t.Error("VDel should return false for nonexistent namespace")
	}
}

func TestVCount(t *testing.T) {
	s := New()

	if s.VCount("docs") != 0 {
		t.Error("VCount should be 0 for empty namespace")
	}

	s.VSet("docs", "chunk:1", 2, []float32{0.1, 0.2}, nil)
	s.VSet("docs", "chunk:2", 2, []float32{0.3, 0.4}, nil)

	if s.VCount("docs") != 2 {
		t.Errorf("expected VCount 2, got %d", s.VCount("docs"))
	}

	s.VDel("docs", "chunk:1")
	if s.VCount("docs") != 1 {
		t.Errorf("expected VCount 1 after delete, got %d", s.VCount("docs"))
	}
}

func TestMultipleNamespaces(t *testing.T) {
	s := New()

	s.VSet("ns1", "a", 2, []float32{1.0, 0.0}, nil)
	s.VSet("ns2", "a", 2, []float32{0.0, 1.0}, nil)

	e1, ok := s.VGet("ns1", "a")
	if !ok || e1.Vector[0] != 1.0 {
		t.Error("ns1 should have [1.0, 0.0]")
	}

	e2, ok := s.VGet("ns2", "a")
	if !ok || e2.Vector[1] != 1.0 {
		t.Error("ns2 should have [0.0, 1.0]")
	}

	// Namespaces are independent
	if s.VCount("ns1") != 1 || s.VCount("ns2") != 1 {
		t.Error("each namespace should have 1 vector")
	}
}

func TestVSnapshot(t *testing.T) {
	s := New()
	s.VSet("docs", "a", 2, []float32{1.0, 0.0}, nil)
	s.VSet("docs", "b", 2, []float32{0.0, 1.0}, nil)

	snap := s.VSnapshot("docs")
	if len(snap) != 2 {
		t.Errorf("expected 2 entries in snapshot, got %d", len(snap))
	}

	// Snapshot of empty namespace
	snap = s.VSnapshot("nonexistent")
	if snap != nil {
		t.Errorf("expected nil for nonexistent namespace, got %v", snap)
	}
}

func TestVSetOverwrite(t *testing.T) {
	s := New()
	s.VSet("docs", "chunk:1", 2, []float32{1.0, 0.0}, map[string]string{"v": "1"})
	s.VSet("docs", "chunk:1", 2, []float32{0.0, 1.0}, map[string]string{"v": "2"})

	entry, ok := s.VGet("docs", "chunk:1")
	if !ok {
		t.Fatal("VGet returned not found after overwrite")
	}
	if entry.Vector[0] != 0.0 || entry.Vector[1] != 1.0 {
		t.Error("vector should be updated to [0.0, 1.0]")
	}
	if entry.Metadata["v"] != "2" {
		t.Error("metadata should be updated to v=2")
	}
	if s.VCount("docs") != 1 {
		t.Error("VCount should still be 1 after overwrite")
	}
}

func TestVSetNilMetadata(t *testing.T) {
	s := New()
	err := s.VSet("docs", "chunk:1", 2, []float32{1.0, 0.0}, nil)
	if err != nil {
		t.Fatalf("VSet with nil metadata should succeed: %v", err)
	}
	entry, _ := s.VGet("docs", "chunk:1")
	if entry.Metadata != nil {
		t.Error("metadata should be nil")
	}
}
