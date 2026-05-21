package similarity

import (
	"errors"
	"math"
)

// Sentinel errors for cosine similarity computation.
var (
	ErrDimensionMismatch = errors.New("dimension mismatch")
	ErrZeroVector        = errors.New("zero-magnitude vector")
)

// CosineSimilarity computes the cosine similarity between two float32 vectors.
// Returns a value in [-1, 1]. Returns an error if dimensions differ or either
// vector has zero magnitude.
func CosineSimilarity(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, ErrDimensionMismatch
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0, ErrZeroVector
	}

	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB)))), nil
}
