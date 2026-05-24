package main

import (
	"encoding/json"
	"math/rand"
	"os"
)

type SampleItem struct {
	ID        string            `json:"id"`
	Text      string            `json:"text"`
	Embedding []float32         `json:"embedding"`
	Metadata  map[string]string `json:"metadata"`
}

func main() {
	texts := []string{
		"uses a sync.RWMutex so multiple goroutines can hold read locks simultaneously...",
		"the connection goroutine reads from its own bufio.Reader without touching...",
		"GOMAXPROCS workers in the pool means reads are distributed across cores...",
		"the AOF log replays every VSET command in sequence on startup...",
		"BGSAVE triggers an async snapshot — the server continues accepting connections...",
		"CRC32 checksum per AOF entry ensures corrupted lines are skipped with a warning...",
	}

	ids := []string{"chunk:42", "chunk:17", "chunk:88", "chunk:31", "chunk:55", "chunk:12"}

	var items []SampleItem
	for i, txt := range texts {
		vec := make([]float32, 1536)
		for j := 0; j < 1536; j++ {
			vec[j] = rand.Float32()
		}
		items = append(items, SampleItem{
			ID:        ids[i],
			Text:      txt,
			Embedding: vec,
			Metadata:  map[string]string{"source": "documentation", "namespace": "docs", "page": "1"},
		})
	}

	f, _ := os.Create("data/sample_embeddings.json")
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.Encode(items)
}
