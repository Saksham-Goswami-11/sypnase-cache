package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/sakshamgoswami/synapse-cache/internal/hnsw"
)

func randomVector(rng *rand.Rand, dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1
	}
	return vec
}

func bruteForceTopK(query []float32, vecs [][]float32, K int) []int {
	type scored struct {
		idx  int
		dist float32
	}
	scores := make([]scored, len(vecs))
	for i, v := range vecs {
		scores[i] = scored{idx: i, dist: hnsw.CosineDistance(query, v)}
	}
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

func main() {
	rng := rand.New(rand.NewSource(42))
	n := 2000 // Test with 2K to be fast
	dim := 1536
	K := 10

	idx := hnsw.NewIndex(16, 200)

	start := time.Now()
	vecs := make([][]float32, n)
	for i := 0; i < n; i++ {
		vecs[i] = randomVector(rng, dim)
		_ = idx.Insert(fmt.Sprintf("vec:%d", i), vecs[i], nil)
	}
	fmt.Printf("Inserted %d in %v\n", n, time.Since(start))

	queries := make([][]float32, 50)
	for i := range queries {
		queries[i] = randomVector(rng, dim)
	}

	for _, ef := range []int{100, 300} {
		totalFound := 0
		for _, query := range queries {
			gtIndices := bruteForceTopK(query, vecs, K)
			gtSet := make(map[string]bool, K)
			for _, gi := range gtIndices {
				gtSet[fmt.Sprintf("vec:%d", gi)] = true
			}

			results, _ := idx.Search(query, K, ef)
			for _, r := range results {
				if gtSet[r.ID] {
					totalFound++
				}
			}
		}
		recall := float64(totalFound) / float64(len(queries)*K)
		fmt.Printf("Recall@%d (n=%d, dim=%d, ef=%d): %.2f%%\n", K, n, dim, ef, recall*100)
	}
}
