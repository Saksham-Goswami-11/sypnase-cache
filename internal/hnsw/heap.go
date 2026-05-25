package hnsw

// Item is a distance-annotated node reference used by the priority queues.
type Item struct {
	ID   uint32
	Dist float32
}

// --- MinHeap: pop returns the item with SMALLEST distance (for candidates) ---

// MinHeap implements container/heap.Interface.
// Used as the candidate queue in SEARCH_LAYER: we always explore
// the closest unvisited node first.
type MinHeap []Item

func (h MinHeap) Len() int            { return len(h) }
func (h MinHeap) Less(i, j int) bool   { return h[i].Dist < h[j].Dist }
func (h MinHeap) Swap(i, j int)        { h[i], h[j] = h[j], h[i] }

func (h *MinHeap) Push(x interface{}) {
	*h = append(*h, x.(Item))
}

func (h *MinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// Peek returns the top element without removing it.
func (h MinHeap) Peek() Item {
	return h[0]
}

// --- MaxHeap: pop returns the item with LARGEST distance (for found set) ---

// MaxHeap implements container/heap.Interface.
// Used as the "found" set in SEARCH_LAYER: we want to quickly
// evict the furthest node when len(found) > ef.
type MaxHeap []Item

func (h MaxHeap) Len() int            { return len(h) }
func (h MaxHeap) Less(i, j int) bool   { return h[i].Dist > h[j].Dist }
func (h MaxHeap) Swap(i, j int)        { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x interface{}) {
	*h = append(*h, x.(Item))
}

func (h *MaxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// Peek returns the top element (largest distance) without removing it.
func (h MaxHeap) Peek() Item {
	return h[0]
}
