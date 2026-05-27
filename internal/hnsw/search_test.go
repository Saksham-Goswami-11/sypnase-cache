package hnsw

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

// --- Helper functions ---

func randomVector(rng *rand.Rand, dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1 // [-1, 1]
	}
	return vec
}

// bruteForceTopK computes exact top-K nearest neighbors by exhaustive scan.
func bruteForceTopK(query []float32, vecs [][]float32, K int) []int {
	type scored struct {
		idx  int
		dist float32
	}
	scores := make([]scored, len(vecs))
	for i, v := range vecs {
		scores[i] = scored{idx: i, dist: CosineDistance(query, v)}
	}

	// Selection sort for K smallest (good enough for test sizes)
	for i := 0; i < K && i < len(scores); i++ {
		minIdx := i
		for j := i + 1; j < len(scores); j++ {
			if scores[j].dist < scores[minIdx].dist {
				minIdx = j
			}
		}
		scores[i], scores[minIdx] = scores[minIdx], scores[i]
	}

	result := make([]int, 0, K)
	for i := 0; i < K && i < len(scores); i++ {
		result = append(result, scores[i].idx)
	}
	return result
}

// --- SEARCH_LAYER Tests ---

func TestSearchLayer_TinyGraph(t *testing.T) {
	// 5 vectors in 2D, hand-built graph
	// Node 0: [1, 0]
	// Node 1: [0.9, 0.1]
	// Node 2: [0, 1]
	// Node 3: [-1, 0]
	// Node 4: [0, -1]
	idx := NewIndex(4, 50)

	vecs := [][]float32{
		{1.0, 0.0},
		{0.9, 0.1},
		{0.0, 1.0},
		{-1.0, 0.0},
		{0.0, -1.0},
	}

	// Manually build nodes
	for i, v := range vecs {
		node := NewNode(uint32(i), v, nil, 0) // all in layer 0
		idx.nodes = append(idx.nodes, node)
		key := fmt.Sprintf("node:%d", i)
		idx.idMap[key] = uint32(i)
		idx.reverseMap = append(idx.reverseMap, key)
	}

	// Connect them in a ring: 0-1-2-3-4-0
	idx.nodes[0].Links[0] = []uint32{1, 4}
	idx.nodes[1].Links[0] = []uint32{0, 2}
	idx.nodes[2].Links[0] = []uint32{1, 3}
	idx.nodes[3].Links[0] = []uint32{2, 4}
	idx.nodes[4].Links[0] = []uint32{3, 0}

	// Query: [0.8, 0.2] — closest by cosine should be node 1 ([0.9, 0.1])
	// cos([0.8,0.2],[0.9,0.1]) ≈ 0.9939 → dist ≈ 0.0061
	// cos([0.8,0.2],[1.0,0.0]) ≈ 0.9701 → dist ≈ 0.0299
	query := []float32{0.8, 0.2}
	found := idx.searchLayer(query, []uint32{0}, 3, 0)

	if found.Len() == 0 {
		t.Fatal("searchLayer returned empty result")
	}

	// Collect all found items and find the nearest
	var nearest Item
	nearest.Dist = float32(math.MaxFloat32)
	for found.Len() > 0 {
		item := found.Pop().(Item)
		if item.Dist < nearest.Dist {
			nearest = item
		}
	}

	if nearest.ID != 1 {
		t.Fatalf("expected nearest=node:1, got node:%d (dist=%.4f)", nearest.ID, nearest.Dist)
	}
}

func TestSearchLayer_100Random2D(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	idx := NewIndex(8, 50)

	n := 100
	dim := 2
	vecs := make([][]float32, n)

	// Build nodes and fully connect each to its 8 nearest neighbors
	for i := 0; i < n; i++ {
		v := randomVector(rng, dim)
		vecs[i] = v
		node := NewNode(uint32(i), v, nil, 0)
		idx.nodes = append(idx.nodes, node)
		key := fmt.Sprintf("node:%d", i)
		idx.idMap[key] = uint32(i)
		idx.reverseMap = append(idx.reverseMap, key)
	}

	// Connect each node to its 8 nearest neighbors
	for i := 0; i < n; i++ {
		nearest := bruteForceTopK(vecs[i], vecs, 9) // include self
		for _, j := range nearest {
			if j != i {
				idx.nodes[i].Links[0] = append(idx.nodes[i].Links[0], uint32(j))
			}
		}
	}

	// Run 50 queries and compare to brute-force
	K := 1
	matches := 0
	total := 50

	for q := 0; q < total; q++ {
		query := randomVector(rng, dim)

		// HNSW search
		found := idx.searchLayer(query, []uint32{0}, 20, 0)
		var hnswNearest uint32
		var hnswBest float32 = float32(math.MaxFloat32)
		for found.Len() > 0 {
			item := found.Pop().(Item)
			if item.Dist < hnswBest {
				hnswBest = item.Dist
				hnswNearest = item.ID
			}
		}

		// Brute-force
		bfNearest := bruteForceTopK(query, vecs, K)

		if hnswNearest == uint32(bfNearest[0]) {
			matches++
		}
	}

	accuracy := float64(matches) / float64(total) * 100
	t.Logf("searchLayer accuracy on 100-node 2D graph: %.1f%% (%d/%d)", accuracy, matches, total)

	// With k-NN connected graph and ef=20, expect decent accuracy
	if accuracy < 70 {
		t.Errorf("expected >90%% accuracy on well-connected 2D graph, got %.1f%%", accuracy)
	}
}

// --- Insert Tests ---

func TestInsertSingle(t *testing.T) {
	idx := NewIndex(16, 200)

	err := idx.Insert("vec:0", []float32{1.0, 0.0, 0.0}, nil)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if idx.Len() != 1 {
		t.Fatalf("expected Len()=1, got %d", idx.Len())
	}

	// Search for the same vector — should return itself with score≈1.0
	results, err := idx.Search([]float32{1.0, 0.0, 0.0}, 1, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].ID != "vec:0" {
		t.Fatalf("expected ID=vec:0, got %s", results[0].ID)
	}

	if results[0].Score < 0.99 {
		t.Fatalf("expected score≈1.0, got %.4f", results[0].Score)
	}
}

func TestInsertDuplicate(t *testing.T) {
	idx := NewIndex(16, 200)

	_ = idx.Insert("vec:0", []float32{1.0, 0.0, 0.0}, map[string]string{"v": "1"})
	_ = idx.Insert("vec:0", []float32{0.0, 1.0, 0.0}, map[string]string{"v": "2"})

	if idx.Len() != 1 {
		t.Fatalf("duplicate insert should not create new node, Len()=%d", idx.Len())
	}

	// Search for the updated vector
	results, err := idx.Search([]float32{0.0, 1.0, 0.0}, 1, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results[0].ID != "vec:0" {
		t.Fatalf("expected vec:0, got %s", results[0].ID)
	}

	if results[0].Score < 0.99 {
		t.Fatalf("updated vector should match, score=%.4f", results[0].Score)
	}
}

func TestInsert10(t *testing.T) {
	idx := NewIndex(4, 50)

	rng := rand.New(rand.NewSource(99))
	for i := 0; i < 10; i++ {
		vec := randomVector(rng, 8)
		err := idx.Insert(fmt.Sprintf("vec:%d", i), vec, nil)
		if err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}

	if idx.Len() != 10 {
		t.Fatalf("expected Len()=10, got %d", idx.Len())
	}

	// Search should return results
	query := randomVector(rng, 8)
	results, err := idx.Search(query, 5, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// Verify results are sorted by score descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Fatalf("results not sorted: [%d].Score=%.4f > [%d].Score=%.4f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

// --- Recall Tests ---

func measureRecall(t *testing.T, idx *Index, vecs [][]float32, queries [][]float32, K int, ef int) float64 {
	t.Helper()

	totalFound := 0
	for _, query := range queries {
		// Ground truth from brute-force
		gtIndices := bruteForceTopK(query, vecs, K)
		gtSet := make(map[string]bool, K)
		for _, gi := range gtIndices {
			gtSet[fmt.Sprintf("vec:%d", gi)] = true
		}

		// HNSW search
		results, err := idx.Search(query, K, ef)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		for _, r := range results {
			if gtSet[r.ID] {
				totalFound++
			}
		}
	}

	return float64(totalFound) / float64(len(queries)*K)
}

func TestRecall_1K_dim128(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	n := 1000
	dim := 128
	K := 10

	idx := NewIndex(16, 200)

	vecs := make([][]float32, n)
	for i := 0; i < n; i++ {
		vecs[i] = randomVector(rng, dim)
		err := idx.Insert(fmt.Sprintf("vec:%d", i), vecs[i], nil)
		if err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}

	// Generate 100 random queries
	queries := make([][]float32, 100)
	for i := range queries {
		queries[i] = randomVector(rng, dim)
	}

	recall := measureRecall(t, idx, vecs, queries, K, 100)
	t.Logf("Recall@%d (1K vectors, dim=%d, ef=100): %.2f%%", K, dim, recall*100)

	if recall < 0.90 {
		t.Errorf("recall@%d = %.2f%%, expected > 90%%", K, recall*100)
	}
}

func TestRecall_1K_dim128_efSweep(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	n := 1000
	dim := 128
	K := 10

	idx := NewIndex(16, 200)

	vecs := make([][]float32, n)
	for i := 0; i < n; i++ {
		vecs[i] = randomVector(rng, dim)
		_ = idx.Insert(fmt.Sprintf("vec:%d", i), vecs[i], nil)
	}

	queries := make([][]float32, 50)
	for i := range queries {
		queries[i] = randomVector(rng, dim)
	}

	efValues := []int{10, 20, 50, 100, 200, 500}
	for _, ef := range efValues {
		recall := measureRecall(t, idx, vecs, queries, K, ef)
		t.Logf("  ef=%3d → recall@%d = %.2f%%", ef, K, recall*100)
	}
}

func TestRecall_10K_dim128(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10K recall test in short mode")
	}

	rng := rand.New(rand.NewSource(42))
	n := 10000
	dim := 128
	K := 10

	idx := NewIndex(16, 200)

	vecs := make([][]float32, n)
	for i := 0; i < n; i++ {
		vecs[i] = randomVector(rng, dim)
		err := idx.Insert(fmt.Sprintf("vec:%d", i), vecs[i], nil)
		if err != nil {
			t.Fatalf("Insert %d failed: %v", i, err)
		}
	}

	queries := make([][]float32, 50)
	for i := range queries {
		queries[i] = randomVector(rng, dim)
	}

	recall100 := measureRecall(t, idx, vecs, queries, K, 100)
	t.Logf("Recall@%d (10K, dim=%d, ef=100): %.2f%%", K, dim, recall100*100)
	if recall100 < 0.80 {
		t.Errorf("recall@%d = %.2f%%, expected > 80%%", K, recall100*100)
	}

	recall300 := measureRecall(t, idx, vecs, queries, K, 300)
	t.Logf("Recall@%d (10K, dim=%d, ef=300): %.2f%%", K, dim, recall300*100)
	if recall300 < 0.93 {
		t.Errorf("recall@%d = %.2f%%, expected > 93%%", K, recall300*100)
	}
}
