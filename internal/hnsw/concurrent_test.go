package hnsw

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestConcurrentInsert_Race(t *testing.T) {
	idx := NewIndex(16, 200)
	var wg sync.WaitGroup

	goroutines := 100
	insertsPerGoroutine := 10
	dim := 128

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(offset)))
			for i := 0; i < insertsPerGoroutine; i++ {
				vec := make([]float32, dim)
				for d := range vec {
					vec[d] = rng.Float32()*2 - 1
				}
				key := fmt.Sprintf("vec:%d", offset*insertsPerGoroutine+i)
				_ = idx.Insert(key, vec, nil)
			}
		}(g)
	}

	wg.Wait()

	expected := goroutines * insertsPerGoroutine
	got := idx.Len()
	if got != expected {
		t.Fatalf("expected %d vectors, got %d", expected, got)
	}
}

func TestConcurrentSearchInsert(t *testing.T) {
	idx := NewIndex(16, 200)
	dim := 128

	// Pre-load some vectors so searches have something to work with
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		vec := make([]float32, dim)
		for d := range vec {
			vec[d] = rng.Float32()*2 - 1
		}
		_ = idx.Insert(fmt.Sprintf("pre:%d", i), vec, nil)
	}

	var wg sync.WaitGroup

	// 50 inserters
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(offset + 1000)))
			for i := 0; i < 10; i++ {
				vec := make([]float32, dim)
				for d := range vec {
					vec[d] = r.Float32()*2 - 1
				}
				_ = idx.Insert(fmt.Sprintf("ins:%d:%d", offset, i), vec, nil)
			}
		}(g)
	}

	// 50 searchers
	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(offset + 2000)))
			for i := 0; i < 10; i++ {
				query := make([]float32, dim)
				for d := range query {
					query[d] = r.Float32()*2 - 1
				}
				_, _ = idx.Search(query, 5, 50)
			}
		}(g)
	}

	wg.Wait()

	// Verify all inserts landed
	expectedMin := 100 + 50*10 // pre-loaded + concurrent inserts
	if idx.Len() < expectedMin {
		t.Fatalf("expected at least %d vectors, got %d", expectedMin, idx.Len())
	}
}

func TestConcurrentInsert_Deadlock(t *testing.T) {
	idx := NewIndex(16, 200)
	dim := 64

	done := make(chan struct{})

	go func() {
		var wg sync.WaitGroup
		for g := 0; g < 50; g++ {
			wg.Add(1)
			go func(offset int) {
				defer wg.Done()
				rng := rand.New(rand.NewSource(int64(offset)))
				for i := 0; i < 20; i++ {
					vec := make([]float32, dim)
					for d := range vec {
						vec[d] = rng.Float32()*2 - 1
					}
					_ = idx.Insert(fmt.Sprintf("vec:%d", offset*20+i), vec, nil)
				}
			}(g)
		}
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		expected := 50 * 20
		if idx.Len() != expected {
			t.Fatalf("expected %d vectors, got %d", expected, idx.Len())
		}
	case <-time.After(60 * time.Second):
		t.Fatal("DEADLOCK: concurrent inserts did not complete within 60 seconds")
	}
}
