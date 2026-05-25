package hnsw

import "math"

// CosineDistance computes 1 - cosineSimilarity(a, b).
// Returns a value in [0, 2] where 0 = identical, 2 = opposite.
// If either vector has zero magnitude, returns 2.0 (maximum distance).
// Panics if len(a) != len(b) — caller must ensure dimension match.
func CosineDistance(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 2.0 // maximum distance for zero vectors
	}

	sim := dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))

	// Clamp to [-1, 1] to handle float precision
	if sim > 1.0 {
		sim = 1.0
	}
	if sim < -1.0 {
		sim = -1.0
	}

	return 1.0 - sim
}
