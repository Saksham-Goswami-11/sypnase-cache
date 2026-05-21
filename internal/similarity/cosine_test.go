package similarity

import (
	"math"
	"testing"
)

func TestCosineSimilarityIdentical(t *testing.T) {
	a := []float32{1, 0, 0}
	score, err := CosineSimilarity(a, a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(float64(score)-1.0) > 1e-6 {
		t.Errorf("identical vectors: expected 1.0, got %f", score)
	}
}

func TestCosineSimilarityOpposite(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{-1, 0, 0}
	score, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(float64(score)+1.0) > 1e-6 {
		t.Errorf("opposite vectors: expected -1.0, got %f", score)
	}
}

func TestCosineSimilarityOrthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	score, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(float64(score)) > 1e-6 {
		t.Errorf("orthogonal vectors: expected 0.0, got %f", score)
	}
}

func TestCosineSimilarityDimensionMismatch(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	_, err := CosineSimilarity(a, b)
	if err != ErrDimensionMismatch {
		t.Errorf("expected ErrDimensionMismatch, got %v", err)
	}
}

func TestCosineSimilarityZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 0, 0}
	_, err := CosineSimilarity(a, b)
	if err != ErrZeroVector {
		t.Errorf("expected ErrZeroVector, got %v", err)
	}
}

func TestCosineSimilarityNonUnitVectors(t *testing.T) {
	// [3, 4] and [4, 3] — both non-unit but well-defined
	a := []float32{3, 4}
	b := []float32{4, 3}
	score, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// cos(theta) = (12 + 12) / (5 * 5) = 24/25 = 0.96
	expected := float32(24.0 / 25.0)
	if math.Abs(float64(score-expected)) > 1e-5 {
		t.Errorf("expected %f, got %f", expected, score)
	}
}
