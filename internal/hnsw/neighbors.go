package hnsw

import "container/heap"

// selectNeighborsHeuristic implements Algorithm 4 from Malkov & Yashunin.
// It selects up to M neighbors from candidates, preferring spatial diversity:
// a candidate is only kept if it is closer to the query than to any
// already-selected neighbor. This prevents neighbors from clustering
// in the same region of vector space.
func (idx *Index) selectNeighborsHeuristic(
	queryVec []float32,
	candidates []Item,
	M int,
	layer int,
	extendCandidates bool,
	keepPruned bool,
) []uint32 {
	if len(candidates) == 0 {
		return nil
	}

	// Build a working min-heap sorted by distance to query
	working := &MinHeap{}
	heap.Init(working)

	seen := make(map[uint32]bool, len(candidates))
	for _, c := range candidates {
		if !seen[c.ID] {
			seen[c.ID] = true
			heap.Push(working, c)
		}
	}

	// Optional: extend candidates by adding neighbors of each candidate
	if extendCandidates {
		for _, c := range candidates {
			idx.mu.RLock()
			if int(c.ID) >= len(idx.nodes) {
				idx.mu.RUnlock()
				continue
			}
			cNode := idx.nodes[c.ID]
			idx.mu.RUnlock()

			neighbors := cNode.GetLinks(layer)
			for _, nID := range neighbors {
				if !seen[nID] {
					seen[nID] = true

					idx.mu.RLock()
					if int(nID) >= len(idx.nodes) {
						idx.mu.RUnlock()
						continue
					}
					nNode := idx.nodes[nID]
					idx.mu.RUnlock()

					dist := CosineDistance(queryVec, nNode.Vector)
					heap.Push(working, Item{ID: nID, Dist: dist})
				}
			}
		}
	}

	// Collect node vectors we'll need for distance checks (avoid repeated locking)
	nodeVecs := make(map[uint32][]float32, M)

	result := make([]uint32, 0, M)
	var discarded []Item

	for working.Len() > 0 && len(result) < M {
		e := heap.Pop(working).(Item)

		// Get the candidate's vector
		eVec, ok := nodeVecs[e.ID]
		if !ok {
			idx.mu.RLock()
			if int(e.ID) >= len(idx.nodes) {
				idx.mu.RUnlock()
				continue
			}
			eVec = idx.nodes[e.ID].Vector
			idx.mu.RUnlock()
			nodeVecs[e.ID] = eVec
		}

		// Check: is e closer to query than to ALL already-selected neighbors?
		isUseful := true
		for _, rID := range result {
			rVec, ok2 := nodeVecs[rID]
			if !ok2 {
				idx.mu.RLock()
				if int(rID) >= len(idx.nodes) {
					idx.mu.RUnlock()
					continue
				}
				rVec = idx.nodes[rID].Vector
				idx.mu.RUnlock()
				nodeVecs[rID] = rVec
			}

			distToSelected := CosineDistance(eVec, rVec)
			if distToSelected < e.Dist {
				isUseful = false
				break
			}
		}

		if isUseful {
			result = append(result, e.ID)
		} else {
			discarded = append(discarded, e)
		}
	}

	// Backfill with pruned candidates if we didn't fill M slots
	if keepPruned {
		dh := &MinHeap{}
		heap.Init(dh)
		for _, d := range discarded {
			heap.Push(dh, d)
		}
		for dh.Len() > 0 && len(result) < M {
			e := heap.Pop(dh).(Item)
			result = append(result, e.ID)
		}
	}

	return result
}

// selectNeighborsSimple takes the M closest candidates. Used as fallback.
func selectNeighborsSimple(candidates []Item, M int) []uint32 {
	h := &MinHeap{}
	heap.Init(h)
	for _, c := range candidates {
		heap.Push(h, c)
	}

	result := make([]uint32, 0, M)
	for h.Len() > 0 && len(result) < M {
		item := heap.Pop(h).(Item)
		result = append(result, item.ID)
	}
	return result
}
