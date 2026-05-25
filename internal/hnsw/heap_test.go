package hnsw

import (
	"container/heap"
	"math"
	"math/rand"
	"sort"
	"testing"
)

// --- MinHeap Tests ---

func TestHeapMinPop(t *testing.T) {
	h := &MinHeap{}
	heap.Init(h)

	// Push 1000 items with random distances
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 1000; i++ {
		heap.Push(h, Item{ID: uint32(i), Dist: rng.Float32() * 100})
	}

	if h.Len() != 1000 {
		t.Fatalf("expected 1000 items, got %d", h.Len())
	}

	// Pop all — must come out in ascending distance order
	prev := float32(-1.0)
	for h.Len() > 0 {
		item := heap.Pop(h).(Item)
		if item.Dist < prev {
			t.Fatalf("MinHeap violated: popped %.4f after %.4f", item.Dist, prev)
		}
		prev = item.Dist
	}
}

func TestHeapMaxPop(t *testing.T) {
	h := &MaxHeap{}
	heap.Init(h)

	// Push 1000 items with random distances
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 1000; i++ {
		heap.Push(h, Item{ID: uint32(i), Dist: rng.Float32() * 100})
	}

	if h.Len() != 1000 {
		t.Fatalf("expected 1000 items, got %d", h.Len())
	}

	// Pop all — must come out in descending distance order
	prev := float32(101.0)
	for h.Len() > 0 {
		item := heap.Pop(h).(Item)
		if item.Dist > prev {
			t.Fatalf("MaxHeap violated: popped %.4f after %.4f", item.Dist, prev)
		}
		prev = item.Dist
	}
}

func TestHeapMinPeek(t *testing.T) {
	h := &MinHeap{}
	heap.Init(h)

	heap.Push(h, Item{ID: 1, Dist: 5.0})
	heap.Push(h, Item{ID: 2, Dist: 2.0})
	heap.Push(h, Item{ID: 3, Dist: 8.0})

	peek := h.Peek()
	if peek.Dist != 2.0 {
		t.Fatalf("MinHeap.Peek() expected dist=2.0, got %.4f", peek.Dist)
	}
	// Peek should not remove the item
	if h.Len() != 3 {
		t.Fatalf("Peek should not remove items, got len=%d", h.Len())
	}
}

func TestHeapMaxPeek(t *testing.T) {
	h := &MaxHeap{}
	heap.Init(h)

	heap.Push(h, Item{ID: 1, Dist: 5.0})
	heap.Push(h, Item{ID: 2, Dist: 2.0})
	heap.Push(h, Item{ID: 3, Dist: 8.0})

	peek := h.Peek()
	if peek.Dist != 8.0 {
		t.Fatalf("MaxHeap.Peek() expected dist=8.0, got %.4f", peek.Dist)
	}
	if h.Len() != 3 {
		t.Fatalf("Peek should not remove items, got len=%d", h.Len())
	}
}

func TestHeapEmpty(t *testing.T) {
	minH := &MinHeap{}
	heap.Init(minH)

	if minH.Len() != 0 {
		t.Fatalf("empty MinHeap should have Len()=0, got %d", minH.Len())
	}

	maxH := &MaxHeap{}
	heap.Init(maxH)

	if maxH.Len() != 0 {
		t.Fatalf("empty MaxHeap should have Len()=0, got %d", maxH.Len())
	}
}

func TestHeapSingleItem(t *testing.T) {
	h := &MinHeap{}
	heap.Init(h)

	heap.Push(h, Item{ID: 42, Dist: 3.14})
	item := heap.Pop(h).(Item)

	if item.ID != 42 || item.Dist != 3.14 {
		t.Fatalf("expected {42, 3.14}, got {%d, %f}", item.ID, item.Dist)
	}
	if h.Len() != 0 {
		t.Fatalf("heap should be empty after popping single item")
	}
}

func TestHeapDuplicateDistances(t *testing.T) {
	h := &MinHeap{}
	heap.Init(h)

	for i := 0; i < 100; i++ {
		heap.Push(h, Item{ID: uint32(i), Dist: 5.0})
	}

	for h.Len() > 0 {
		item := heap.Pop(h).(Item)
		if item.Dist != 5.0 {
			t.Fatalf("all distances should be 5.0, got %.4f", item.Dist)
		}
	}
}

func TestHeapMinFixAfterUpdate(t *testing.T) {
	h := &MinHeap{
		{ID: 0, Dist: 10.0},
		{ID: 1, Dist: 20.0},
		{ID: 2, Dist: 30.0},
	}
	heap.Init(h)

	// Update the root to a larger value
	(*h)[0].Dist = 25.0
	heap.Fix(h, 0)

	item := heap.Pop(h).(Item)
	if item.Dist != 20.0 {
		t.Fatalf("after Fix, expected min=20.0, got %.4f", item.Dist)
	}
}

// --- randomLevel Tests ---

func TestRandomLevel_Distribution(t *testing.T) {
	idx := NewIndex(16, 200)

	const n = 100000
	counts := make(map[int]int)

	for i := 0; i < n; i++ {
		l := idx.randomLevel()
		counts[l]++
	}

	// With M=16, mL = 1/ln(16) ≈ 0.3607
	// level = floor(-ln(U) * mL)
	// P(level=0) = P(-ln(U) * mL < 1) = P(U > exp(-1/mL)) = 1 - 1/M ≈ 93.75%
	// P(level=1) = 1/M - 1/M² ≈ 5.86%
	// P(level=2) ≈ 0.37%
	level0Pct := float64(counts[0]) / float64(n) * 100
	level1Pct := float64(counts[1]) / float64(n) * 100

	// Allow ±3% tolerance
	if math.Abs(level0Pct-93.75) > 3 {
		t.Errorf("P(level=0) = %.1f%%, expected ~93.75%% (±3%%)", level0Pct)
	}
	if math.Abs(level1Pct-5.86) > 3 {
		t.Errorf("P(level=1) = %.1f%%, expected ~5.86%% (±3%%)", level1Pct)
	}

	// Verify monotonically decreasing: each level should have fewer nodes
	maxL := 0
	for l := range counts {
		if l > maxL {
			maxL = l
		}
	}
	for l := 1; l <= maxL; l++ {
		if counts[l] > counts[l-1] {
			t.Errorf("level %d count (%d) > level %d count (%d) — should be decreasing",
				l, counts[l], l-1, counts[l-1])
		}
	}

	t.Logf("Level distribution (n=%d):", n)
	levels := make([]int, 0, len(counts))
	for l := range counts {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	for _, l := range levels {
		t.Logf("  level %d: %d (%.2f%%)", l, counts[l], float64(counts[l])/float64(n)*100)
	}
}

func TestRandomLevel_NeverNegative(t *testing.T) {
	idx := NewIndex(16, 200)
	for i := 0; i < 10000; i++ {
		l := idx.randomLevel()
		if l < 0 {
			t.Fatalf("randomLevel() returned negative: %d", l)
		}
	}
}

func TestRandomLevel_RespectsCap(t *testing.T) {
	idx := NewIndex(16, 200)
	idx.MaxLevel = 5

	for i := 0; i < 50000; i++ {
		l := idx.randomLevel()
		if l > 5 {
			t.Fatalf("randomLevel() exceeded MaxLevel cap: got %d, cap %d", l, idx.MaxLevel)
		}
	}
}

// --- CosineDistance Tests ---

func TestCosineDistance_Identical(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	dist := CosineDistance(a, a)
	if dist > 1e-6 {
		t.Fatalf("distance between identical vectors should be ~0, got %f", dist)
	}
}

func TestCosineDistance_Opposite(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{-1.0, 0.0}
	dist := CosineDistance(a, b)
	if math.Abs(float64(dist)-2.0) > 1e-6 {
		t.Fatalf("distance between opposite vectors should be ~2.0, got %f", dist)
	}
}

func TestCosineDistance_Orthogonal(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{0.0, 1.0}
	dist := CosineDistance(a, b)
	if math.Abs(float64(dist)-1.0) > 1e-6 {
		t.Fatalf("distance between orthogonal vectors should be ~1.0, got %f", dist)
	}
}

func TestCosineDistance_ZeroVector(t *testing.T) {
	a := []float32{1.0, 0.0}
	b := []float32{0.0, 0.0}
	dist := CosineDistance(a, b)
	if dist != 2.0 {
		t.Fatalf("distance with zero vector should be 2.0, got %f", dist)
	}
}

// --- Node Tests ---

func TestNewNode(t *testing.T) {
	meta := map[string]string{"source": "test.pdf"}
	n := NewNode(42, []float32{1.0, 2.0, 3.0}, meta, 3)

	if n.ID != 42 {
		t.Fatalf("expected ID=42, got %d", n.ID)
	}
	if n.MaxLayer() != 3 {
		t.Fatalf("expected MaxLayer=3, got %d", n.MaxLayer())
	}
	if len(n.Links) != 4 { // layers 0, 1, 2, 3
		t.Fatalf("expected 4 layer slices, got %d", len(n.Links))
	}
	for i, links := range n.Links {
		if len(links) != 0 {
			t.Fatalf("layer %d should start empty, has %d links", i, len(links))
		}
	}
}

func TestNode_AddAndGetLinks(t *testing.T) {
	n := NewNode(0, []float32{1.0}, nil, 2)

	n.AddLink(0, 1)
	n.AddLink(0, 2)
	n.AddLink(1, 3)

	l0 := n.GetLinks(0)
	if len(l0) != 2 || l0[0] != 1 || l0[1] != 2 {
		t.Fatalf("layer 0 links: expected [1,2], got %v", l0)
	}

	l1 := n.GetLinks(1)
	if len(l1) != 1 || l1[0] != 3 {
		t.Fatalf("layer 1 links: expected [3], got %v", l1)
	}

	l2 := n.GetLinks(2)
	if len(l2) != 0 {
		t.Fatalf("layer 2 links: expected [], got %v", l2)
	}
}

func TestNode_GetLinksReturnsCopy(t *testing.T) {
	n := NewNode(0, []float32{1.0}, nil, 0)
	n.AddLink(0, 1)
	n.AddLink(0, 2)

	links := n.GetLinks(0)
	links[0] = 999 // mutate the copy

	original := n.GetLinks(0)
	if original[0] == 999 {
		t.Fatal("GetLinks should return a copy, not a reference to the internal slice")
	}
}

func TestNode_SetLinks(t *testing.T) {
	n := NewNode(0, []float32{1.0}, nil, 1)
	n.AddLink(0, 1)
	n.AddLink(0, 2)
	n.AddLink(0, 3)

	// Replace with pruned list
	n.SetLinks(0, []uint32{1, 3})

	links := n.GetLinks(0)
	if len(links) != 2 || links[0] != 1 || links[1] != 3 {
		t.Fatalf("after SetLinks, expected [1,3], got %v", links)
	}
}

func TestNode_LinkCount(t *testing.T) {
	n := NewNode(0, []float32{1.0}, nil, 2)
	n.AddLink(0, 1)
	n.AddLink(0, 2)
	n.AddLink(1, 3)

	if n.LinkCount() != 3 {
		t.Fatalf("expected LinkCount=3, got %d", n.LinkCount())
	}
}

func TestNode_GetLinksInvalidLayer(t *testing.T) {
	n := NewNode(0, []float32{1.0}, nil, 1)
	links := n.GetLinks(5)
	if links != nil {
		t.Fatalf("expected nil for invalid layer, got %v", links)
	}
}

// --- Index Creation Tests ---

func TestNewIndex_Defaults(t *testing.T) {
	idx := NewIndex(16, 200)

	if idx.M != 16 {
		t.Fatalf("expected M=16, got %d", idx.M)
	}
	if idx.Mmax0 != 32 {
		t.Fatalf("expected Mmax0=32, got %d", idx.Mmax0)
	}
	if idx.EfConstruction != 200 {
		t.Fatalf("expected EfConstruction=200, got %d", idx.EfConstruction)
	}
	if idx.Ef != 100 {
		t.Fatalf("expected Ef=100, got %d", idx.Ef)
	}
	if idx.Len() != 0 {
		t.Fatalf("new index should be empty, got Len()=%d", idx.Len())
	}
}

func TestNewIndex_ZeroParams(t *testing.T) {
	idx := NewIndex(0, 0)

	if idx.M != 16 {
		t.Fatalf("zero M should default to 16, got %d", idx.M)
	}
	if idx.EfConstruction != 200 {
		t.Fatalf("zero efC should default to 200, got %d", idx.EfConstruction)
	}
}

func TestSetEF(t *testing.T) {
	idx := NewIndex(16, 200)

	idx.SetEF(300)
	if idx.GetEF() != 300 {
		t.Fatalf("expected EF=300, got %d", idx.GetEF())
	}

	// Invalid ef should be ignored
	idx.SetEF(0)
	if idx.GetEF() != 300 {
		t.Fatalf("SetEF(0) should be ignored, EF=%d", idx.GetEF())
	}

	idx.SetEF(-5)
	if idx.GetEF() != 300 {
		t.Fatalf("SetEF(-5) should be ignored, EF=%d", idx.GetEF())
	}
}
