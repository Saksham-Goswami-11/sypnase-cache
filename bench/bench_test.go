package bench

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/sakshamgoswami/synapse-cache/internal/similarity"
	"github.com/sakshamgoswami/synapse-cache/internal/store"
)

func randomVector(dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rand.Float32()*2 - 1
	}
	return vec
}

func buildStore(b *testing.B, numVectors, dim int) *store.Store {
	b.Helper()
	s := store.New()
	for i := 0; i < numVectors; i++ {
		vec := randomVector(dim)
		s.VSet("bench", fmt.Sprintf("vec:%d", i), dim, vec, nil)
	}
	return s
}

func BenchmarkCosineSimilarity1536(b *testing.B) {
	a := randomVector(1536)
	bv := randomVector(1536)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		similarity.CosineSimilarity(a, bv)
	}
}

func BenchmarkVSimilarity1K(b *testing.B) {
	s := buildStore(b, 1000, 1536)
	engine := similarity.NewEngine(0)
	query := randomVector(1536)
	entries := s.VSnapshot("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.TopK(query, entries, 10)
	}
}

func BenchmarkVSimilarity10K(b *testing.B) {
	s := buildStore(b, 10000, 1536)
	engine := similarity.NewEngine(0)
	query := randomVector(1536)
	entries := s.VSnapshot("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.TopK(query, entries, 10)
	}
}

func BenchmarkVSimilarity100K(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping 100K benchmark in short mode")
	}
	s := buildStore(b, 100000, 1536)
	engine := similarity.NewEngine(0)
	query := randomVector(1536)
	entries := s.VSnapshot("bench")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.TopK(query, entries, 10)
	}
}

func BenchmarkSetGet(b *testing.B) {
	s := store.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key:%d", i)
		s.Set(key, "value_100_bytes_long_padding_to_reach_one_hundred_bytes_total_for_this_benchmark_testxxx")
		s.Get(key)
	}
}

func BenchmarkVSet1536(b *testing.B) {
	s := store.New()
	vec := randomVector(1536)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.VSet("bench", fmt.Sprintf("vec:%d", i), 1536, vec, nil)
	}
}
