package store

import (
	"fmt"
	"sync"
)

// VectorEntry holds a single vector with its ID and metadata.
type VectorEntry struct {
	ID       string
	Vector   []float32
	Metadata map[string]string
}

// VectorNamespace is a named partition of vectors.
type VectorNamespace struct {
	entries map[string]*VectorEntry
	mu      sync.RWMutex
}

// newVectorNamespace creates an empty namespace.
func newVectorNamespace() *VectorNamespace {
	return &VectorNamespace{
		entries: make(map[string]*VectorEntry),
	}
}

// --- Vector Store Operations (on the main Store) ---

// VSet stores a vector in the given namespace.
// Returns an error if the provided float count doesn't match dim.
func (s *Store) VSet(namespace, id string, dim int, vec []float32, meta map[string]string) error {
	if len(vec) != dim {
		return fmt.Errorf("dimension mismatch: declared %d, got %d floats", dim, len(vec))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ns, ok := s.vectors[namespace]
	if !ok {
		ns = newVectorNamespace()
		s.vectors[namespace] = ns
	}

	// Copy the vector to own the memory
	vecCopy := make([]float32, len(vec))
	copy(vecCopy, vec)

	// Copy metadata
	var metaCopy map[string]string
	if len(meta) > 0 {
		metaCopy = make(map[string]string, len(meta))
		for k, v := range meta {
			metaCopy[k] = v
		}
	}

	ns.entries[id] = &VectorEntry{
		ID:       id,
		Vector:   vecCopy,
		Metadata: metaCopy,
	}
	return nil
}

// VGet retrieves a vector entry from a namespace.
func (s *Store) VGet(namespace, id string) (*VectorEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.vectors[namespace]
	if !ok {
		return nil, false
	}
	entry, ok := ns.entries[id]
	return entry, ok
}

// VDel removes a vector from a namespace. Returns true if it existed.
func (s *Store) VDel(namespace, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, ok := s.vectors[namespace]
	if !ok {
		return false
	}
	if _, ok := ns.entries[id]; !ok {
		return false
	}
	delete(ns.entries, id)
	return true
}

// VCount returns the number of vectors in a namespace.
func (s *Store) VCount(namespace string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.vectors[namespace]
	if !ok {
		return 0
	}
	return len(ns.entries)
}

// VSnapshot returns a snapshot of all vector entries in a namespace.
// The slice headers are copied so similarity computation can happen
// outside the lock without racing with concurrent writes.
func (s *Store) VSnapshot(namespace string) []*VectorEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.vectors[namespace]
	if !ok {
		return nil
	}

	snapshot := make([]*VectorEntry, 0, len(ns.entries))
	for _, entry := range ns.entries {
		snapshot = append(snapshot, entry)
	}
	return snapshot
}

// VectorNamespaces returns the names of all vector namespaces (for INFO).
func (s *Store) VectorNamespaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.vectors))
	for name := range s.vectors {
		names = append(names, name)
	}
	return names
}

// TotalVectors returns the total number of vectors across all namespaces.
func (s *Store) TotalVectors() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := 0
	for _, ns := range s.vectors {
		total += len(ns.entries)
	}
	return total
}
