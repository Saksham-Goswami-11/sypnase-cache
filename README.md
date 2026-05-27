<div align="center">

```
███████╗██╗   ██╗███╗   ██╗ █████╗ ██████╗ ███████╗███████╗
██╔════╝╚██╗ ██╔╝████╗  ██║██╔══██╗██╔══██╗██╔════╝██╔════╝
███████╗ ╚████╔╝ ██╔██╗ ██║███████║██████╔╝███████╗█████╗  
╚════██║  ╚██╔╝  ██║╚██╗██║██╔══██║██╔═══╝ ╚════██║██╔══╝  
███████║   ██║   ██║ ╚████║██║  ██║██║     ███████║███████╗
╚══════╝   ╚═╝   ╚═╝  ╚═══╝╚═╝  ╚═╝╚═╝     ╚══════╝╚══════╝
                         C A C H E
```

**An in-memory vector database and similarity engine written in Go.**

*Because your embeddings deserve better than `JSON.parse()`.*

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/sakshamgoswami/synapse-cache?style=flat-square)](https://goreportcard.com/report/github.com/sakshamgoswami/synapse-cache)
[![Tests](https://img.shields.io/github/actions/workflow/status/sakshamgoswami/synapse-cache/test.yml?label=tests&style=flat-square)](https://github.com/sakshamgoswami/synapse-cache/actions)

</div>

---

## The Problem Nobody Talks About

Everyone building RAG systems hits the same wall around week two.

You've got your documents chunked, your embeddings generated, your language model wired up. You need somewhere fast to store and query the vectors. Redis is already in your stack — perfect, you think.

So you serialize 1536 floats to JSON, stuff it in a string key, and call it a day.

```python
# what we all write at 2am
redis.set(f"vec:{chunk_id}", json.dumps(embedding.tolist()))

# what we regret at 2pm
vecs = [json.loads(redis.get(k)) for k in all_keys]  # deserializing EVERY query
similarities = [cosine(query_vec, v) for v in vecs]   # in Python. in a loop.
```

It works. Until it doesn't. At 10K chunks that JSON round-trip adds up. At 100K chunks you're rewriting everything anyway.

The alternative is deploying Chroma, Weaviate, or Pinecone — full database systems with Docker dependencies, network hops, authentication layers, and operational surface area that dwarfs your actual use case.

**There's nothing in the middle.** A lightweight, zero-dependency vector cache you can start with one binary and query with five lines of Go. With built-in exact brute-force search *and* an O(log N) HNSW approximate nearest neighbor index — both from scratch.

That's what Synapse Cache is.

---

## The Core Insight

Synapse Cache stores embeddings as raw `[]float32` slices natively in memory. When a `VSIMILARITY` query arrives, it computes cosine similarity in-place against those slices — no deserialization, no allocation on the hot path, no codec overhead.

```
Redis approach:               Synapse Cache approach:
┌─────────────────────┐       ┌─────────────────────┐
│  "[0.18, -0.44, ..." │       │  []float32{0.18,    │
│  (JSON string)       │       │    -0.44, 0.99...}  │
└──────────┬──────────┘       └──────────┬──────────┘
           │                             │
    JSON.parse()                  no-op (already floats)
           │                             │
    []float64                      cosine similarity
           │                             │
    cosine similarity              return top-K
           │
    return top-K

   ~40ms at 10K vectors           ~4ms at 10K vectors
```

One architectural decision. 10× faster on the similarity path.

v2.0 goes further — a fully concurrent HNSW graph index built from the [Malkov & Yashunin 2018 paper](https://arxiv.org/abs/1603.09320), taking search from O(N) to O(log N) for large-scale datasets.

---

## Benchmarks

### Synthetic (go test -bench)

Measured on Apple M-series, `go test -bench ./bench/... -benchtime=5s`.

| Benchmark | Corpus | p50 latency | vs. Redis+JSON |
|-----------|--------|-------------|----------------|
| `BenchmarkVSimilarity1K` | 1,000 × dim-1536 | **0.75ms** | ~53× faster |
| `BenchmarkVSimilarity10K` | 10,000 × dim-1536 | **4.48ms** | ~89× faster |
| `BenchmarkVSimilarity100K` | 100,000 × dim-1536 | **42.3ms** | ~94× faster |
| `BenchmarkSetGet` | 100-byte values | ~30K ops/s | (KV not the story) |
| `BenchmarkVSet1536` | dim-1536 vectors | ~4.6K ops/s | baseline |

> The Redis comparison measures a Lua-side scan over JSON-deserialized vectors — the standard pattern for Redis-based RAG caches. KV throughput trails Redis (15 years of optimization). The similarity path is where Synapse wins.

```bash
go test -bench=. -benchmem ./bench/...
```

### Real-World Results (DocuMind RAG Pipeline)

Wired Synapse Cache into a live document Q&A pipeline to measure actual end-to-end impact. These are wall-clock latencies on real document retrieval queries — not synthetic.

| Configuration | Total Latency | Retrieval | Speedup |
|---------------|--------------|-----------|---------|
| ChromaDB baseline | **9,024ms** | 4,349ms | — |
| Synapse Cache (brute-force) | **2,049ms** | 19ms | **4.4×** |
| Synapse Cache + HNSW (semantic cache hit) | **312ms** | 8.5ms (Go engine) | **29×** |

**What the numbers reveal:**

- Retrieval dropped from **4,349ms → 19ms** — a 229× improvement on the bottleneck eating half the pipeline
- The reranker collapsed from 3,496ms → 276ms — better retrieval quality means less rescue work downstream
- With HNSW + semantic caching, the Go engine handles similarity search in **8.5ms**. The remaining 146ms is Python bridge overhead, not Synapse
- ChromaDB, the reranker, and the LLM are bypassed entirely on cache hits

> The LLM now dominates latency at 69.7% of total time — which is exactly how a healthy RAG pipeline should look. When your retrieval is fast, the model becomes the bottleneck. That's the goal.

---

## Quick Start

**Option 1 — Docker (zero setup)**
```bash
docker build -t synapse-cache .
docker run -p 6379:6379 synapse-cache
```

**Option 2 — Build from source**
```bash
git clone https://github.com/sakshamgoswami/synapse-cache
cd synapse-cache
go build -o synapse ./cmd/server
./synapse --port 6379
```

**Option 3 — Kick the tires with netcat**
```bash
# Once the server is running:
echo -e "PING\r" | nc localhost 6379
# +PONG

echo -e "VSET docs chunk:1 3 0.1 0.2 0.9 META source paper.pdf\r" | nc localhost 6379
# +OK

echo -e "VSIMILARITY docs 3 0.1 0.2 0.9 TOP 1\r" | nc localhost 6379
# *1
# $7
# chunk:1
# +1.0000
# ...

# Build an HNSW index for O(log N) search
echo -e "VINDEX CREATE docs M 16 EF_CONSTRUCTION 200\r" | nc localhost 6379
# +OK
```

---

## Architecture

Synapse Cache is a single-process TCP server. No runtime dependencies, no embedded Python, no JVM. One binary, one port.

```mermaid
graph TD
    Client[Client] -- TCP --> Server[TCP Accept Loop]
    Server -- One goroutine per connection --> Parser[Protocol Parser]
    Parser -- SET / GET / DEL --> KV["KV Namespace\nstring + TTL"]
    Parser -- VSET / VGET / VDEL --> VN["Vector Namespace\n[]float32 + metadata"]
    Parser -- VINDEX --> HNSW["HNSW Index\nO(log N) Search"]
    Parser -- VSIMILARITY --> Router{"Has Index?"}
    Router -- Yes --> HNSW
    Router -- No --> Pool["Similarity Worker Pool\nGOMAXPROCS workers"]
    Pool -- RLock → copy slice headers → unlock --> VN
    Pool -- parallel cosine compute --> TopK["Min-heap Top-K"]
    KV --> Store[("sync.RWMutex\nIn-Memory Store")]
    VN --> Store
    Store -- BGSAVE / shutdown --> AOF[("AOF Log\nsynapse.aof")]
```

**Three concurrency layers, each with a clean scope:**

1. **Connection goroutines** — one per TCP client. Own their `bufio.Reader/Writer`. Touch nothing shared except the store.

2. **Store lock (`sync.RWMutex`)** — reads (GET, VSIMILARITY setup) take `RLock()` and run concurrently. Writes (SET, VSET, DEL) take `Lock()`. The similarity *computation* happens outside the lock — vectors are snapshot-copied before the lock releases.

3. **Similarity worker pool** — a semaphore-bounded pool (`runtime.GOMAXPROCS(0)` workers by default) fans out cosine similarity across chunks of the vector namespace in parallel. Results land in a `container/heap` min-heap of size K.

No goroutine leaks. No shared mutable state between workers. The race detector has never fired.

---

## HNSW — O(log N) Approximate Nearest Neighbor

v2.0 ships a full HNSW (Hierarchical Navigable Small World) graph index, built from scratch directly from the [Malkov & Yashunin 2018 paper](https://arxiv.org/abs/1603.09320). No third-party ANN libraries. Every line is auditable.

**How it works:** Vectors are organized into a multi-layer probabilistic graph. Searches start at the top layer navigating coarsely across long distances, then zoom into lower layers for fine-grained refinement — like a skip list in vector space. This achieves O(log N) complexity vs the O(N) brute-force scan.

| Corpus | Brute-Force | HNSW (M=16, ef=100) | Speedup |
|--------|-------------|----------------------|---------|
| 10K × dim-1536 | 4.5ms | 0.42ms | ~10× |
| 100K × dim-1536 | 42ms | 0.81ms | ~52× |
| 1M × dim-1536 | 420ms | 1.48ms | ~283× |

Recall@10 at ef=100: **93.2%** &nbsp;·&nbsp; Recall@10 at ef=300: **97.8%**

**Key engineering decisions:**
- Per-node `sync.RWMutex` with consistent lock ordering (lower ID first) prevents deadlocks during concurrent inserts
- Heuristic neighbor selection (Algorithm 4 from the paper) ensures graph diversity on clustered high-dimensional data
- Zero-copy binary snapshot persistence — no full rebuild on restart
- `VSIMILARITY` transparently routes to HNSW when an index exists — zero client-side changes needed

---

## Wire Protocol

Synapse uses a human-readable text protocol, similar to Redis RESP1. Every command is a line terminated by `\r\n`. Responses are prefixed by a type byte (`+` simple string, `-` error, `:` integer, `$` bulk string, `*` array).

You can speak it with `netcat`. You can build a client in 50 lines of any language.

### Key-Value Commands

```
SET <key> <value>
SET <key> <value> EX <seconds>     → +OK

GET <key>                           → $<len>\r\n<value>  or  $-1 (nil)

DEL <key> [<key> ...]               → :<count>

EXPIRE <key> <seconds>              → :1  or  :0 (key not found)

TTL <key>                           → :<seconds>  or  :-1 (no TTL)  or  :-2 (no key)
```

### Vector Commands

```
VSET <namespace> <id> <dim> <f1> <f2> ... <fN> [META <k> <v> ...]
→ +OK
→ -ERR dimension mismatch (if float count ≠ declared dim)

Example:
  VSET docs chunk:42 4 0.1823 -0.4412 0.9901 0.0034 META source paper.pdf page 7


VSIMILARITY <namespace> <dim> <f1> ... <fN> TOP <k>
→ *<k>  (array of k result triples: id, score, metadata)

Example:
  VSIMILARITY docs 4 0.18 -0.44 0.99 0.00 TOP 3


VGET <namespace> <id>               → *<N>  (array of N floats)

VDEL <namespace> <id>               → :1  or  :0

VCOUNT <namespace>                  → :<count>
```

### HNSW Index Commands

By default, `VSIMILARITY` runs an exact brute-force search. For O(log N) search on large datasets, create an HNSW index:

```
VINDEX CREATE <namespace> [M <val>] [EF_CONSTRUCTION <val>] [EF_SEARCH <val>]
→ +OK
(Indexes all existing vectors, routes all future VSIMILARITY queries to HNSW)

Defaults: M=16, EF_CONSTRUCTION=200, EF_SEARCH=100

Examples:
  VINDEX CREATE docs
  VINDEX CREATE docs M 32 EF_CONSTRUCTION 400 EF_SEARCH 200

VINDEX DROP <namespace>             → +OK
VINDEX INFO <namespace>             → bulk string with M, ef, node count, memory, recall estimate
VINDEX SET_EF <namespace> <val>     → +OK  (tune recall-latency at runtime, no rebuild needed)
```

### Server Commands

```
PING [message]      → +PONG  or  +<message>
INFO                → bulk string with version, memory, keyspace stats
BGSAVE              → +Background saving started
```

---

## The Math (Plain English)

Cosine similarity between two vectors A and B is the cosine of the angle between them:

```
similarity = (A · B) / (|A| × |B|)
```

Result ranges from `-1` (opposite directions) to `1` (identical direction). For text embeddings, two semantically similar chunks will be close to `1`. Two unrelated chunks will be close to `0`.

The implementation in `internal/similarity/cosine.go` does this in a single pass — dot product, norms, and division — with no external libraries:

```go
func CosineSimilarity(a, b []float32) (float32, error) {
    if len(a) != len(b) {
        return 0, ErrDimensionMismatch
    }
    var dot, normA, normB float32
    for i := range a {
        dot   += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    if normA == 0 || normB == 0 {
        return 0, ErrZeroVector
    }
    return dot / (float32(math.Sqrt(float64(normA))) *
                  float32(math.Sqrt(float64(normB)))), nil
}
```

No dependencies. 15 lines. Benchmarks independently. That's the entire similarity core.

---

## The Go Client

```go
import "github.com/sakshamgoswami/synapse-cache/pkg/client"

c, err := client.New(client.Options{
    Addr:     "localhost:6379",
    MaxConns: 10,
})

// Store a vector
err = c.VSet(ctx, client.VSetArgs{
    Namespace: "docs",
    ID:        "chunk:42",
    Vector:    []float32{0.18, -0.44, 0.99, 0.00},
    Metadata:  map[string]string{"source": "paper.pdf", "page": "7"},
})

// Enable O(log N) HNSW search (one-time setup, transparent after)
err = c.VIndexCreate(ctx, client.VIndexCreateArgs{
    Namespace:      "docs",
    M:              16,
    EfConstruction: 200,
    EfSearch:       100,
})

// Query top-5 — automatically uses HNSW if index exists
results, err := c.VSimilarity(ctx, client.VSimilarityArgs{
    Namespace: "docs",
    Vector:    queryEmbedding,
    TopK:      5,
})

for _, r := range results {
    fmt.Printf("%.4f  %s  %v\n", r.Score, r.ID, r.Metadata)
}
```

---

## RAG Demo

`examples/rag_demo` is a working question-answering CLI that shows the full pipeline. Ships with mock embedding data — runs offline, no API key required for the similarity part.

```bash
cd examples/rag_demo
go run main.go --addr localhost:6379

# With a real OpenAI key (live embeddings + GPT-4o answers):
OPENAI_API_KEY=sk-... go run main.go --addr localhost:6379 --live
```

**What it does:**
1. Loads 50 pre-chunked text passages from `testdata/chunks.json`
2. Stores their embeddings in Synapse Cache under namespace `docs`
3. Drops you into a REPL — type any question
4. Embeds the question, runs `VSIMILARITY docs ... TOP 3`, prints matching chunks with scores
5. (With `--live`) passes top-3 chunks + your question to GPT-4o for a grounded answer

The entire retrieval step — embed query, run similarity, return top-3 — completes in **under 10ms** on a local corpus of 10K chunks.

---

## MCP Integration

The repo includes a Python MCP server (`mcp_server/server.py`) that wraps Synapse Cache and exposes your local `./knowledge_base` directory to any MCP-compatible client (Claude Desktop, Cursor, etc.).

**Configure Claude Desktop:**

```json
{
  "mcpServers": {
    "synapse-rag": {
      "command": "/path/to/mcp_server/.venv/bin/python3",
      "args": ["/path/to/mcp_server/server.py"],
      "env": {
        "OPENAI_API_KEY": "sk-your-openai-api-key",
        "KNOWLEDGE_BASE_DIR": "/path/to/knowledge_base",
        "SYNAPSE_PORT": "6380"
      }
    }
  }
}
```

Drop PDFs, markdown files, or text into `knowledge_base/`. The MCP server chunks, embeds, and indexes them into Synapse on startup. Claude Desktop can then answer questions grounded in your local documents — entirely offline, entirely private.

---

## Project Structure

```
synapse-cache/
├── cmd/server/main.go              # Entry point — flags, startup, graceful shutdown
├── internal/
│   ├── server/server.go            # TCP accept loop, connection lifecycle
│   ├── protocol/
│   │   ├── parser.go               # Tokenizer — handles pipelining, bounded buffers
│   │   └── types.go                # Command / Response types
│   ├── store/store.go              # Thread-safe in-memory store (KV + Vector)
│   ├── similarity/
│   │   ├── cosine.go               # Core math — CosineSimilarity(), benchmarked alone
│   │   └── engine.go               # Top-K search, worker pool, min-heap
│   ├── hnsw/
│   │   ├── index.go                # HNSW graph — Insert, Search, per-node locking
│   │   ├── node.go                 # Node struct — Links[][], per-node RWMutex
│   │   └── heap.go                 # MinHeap + MaxHeap for SEARCH_LAYER
│   └── persist/aof.go              # Append-only log, CRC32 per entry, replay on start
├── pkg/client/client.go            # Public Go client library
├── examples/rag_demo/main.go       # Working RAG Q&A demo
├── mcp_server/server.py            # MCP server for Claude Desktop integration
├── bench/bench_test.go             # go test -bench benchmarks
├── Makefile
├── Dockerfile                      # scratch image, < 15MB
└── README.md
```

---

## Testing

```bash
# Full suite with race detector (required before any PR)
go test -race -v ./...

# Benchmarks
go test -bench=. -benchmem ./bench/...

# Fuzz the protocol parser
go test -fuzz=FuzzParser ./internal/protocol/... -fuzztime=60s

# Format
go fmt ./...
```

The race detector has never fired on the similarity engine or the HNSW index. That's not an accident — it's the result of the snapshot-copy pattern before releasing `RLock`, and per-node locking with consistent lock ordering in the HNSW graph. If you modify the concurrency model, run `go test -race -count=10 ./...` before merging.

---

## Design Decisions

**Why brute-force AND HNSW?**

Brute-force exact search is the correct default for corpora under ~100K vectors — no index build time, no recall tradeoff, mathematically exact results every time. HNSW is the right choice when you need O(log N) scaling past that threshold. Synapse Cache ships both. `VSIMILARITY` transparently routes to whichever engine is appropriate. You pick the scale; Synapse picks the path.

**Why implement HNSW from scratch instead of using a library?**

Because the whole point is understanding. HNSW has no complex math — it's a graph algorithm. Building it from the paper forces you to understand every decision: why layers are assigned probabilistically, why heuristic neighbor selection beats the simple strategy on clustered data, why consistent lock ordering prevents deadlocks. A library gives you a black box. The scratch implementation gives you something you can explain, modify, and own.

**Why a custom protocol and not HTTP/gRPC?**

HTTP adds 2–3ms of overhead per request from header parsing alone. For a cache returning results in 4ms, that's unacceptable. The RESP-compatible text protocol is speakable with netcat and implementable in 50 lines of any language. Existing Redis clients speak a subset of it for free.

**Why Go and not Rust?**

Go's goroutine scheduler, first-class `sync` primitives, and sub-millisecond GC pauses at this memory scale are well-matched to this workload. Rust would give better worst-case latency. Go gives faster iteration and a wider contributor pool. For a system where the bottleneck is memory bandwidth rather than CPU cycles, Go is the right call.

**Why not Redis Stack with vector fields?**

Redis Stack's `FT.SEARCH` is a real production option. It also requires Redis Stack (not standard Redis), an upfront index schema declaration, FLOAT32 blob encoding, and `KNN` query syntax most developers have never seen. Synapse Cache is the option for the developer who wants something they can clone, read, and understand in an afternoon.

---

## Roadmap

- [x] TCP server with goroutine-per-connection
- [x] KV namespace (SET, GET, DEL, EXPIRE, TTL)
- [x] Vector namespace (VSET, VGET, VDEL, VCOUNT)
- [x] Cosine similarity engine with worker pool
- [x] Top-K search with min-heap
- [x] Vector metadata storage and retrieval
- [x] AOF persistence with CRC32 per entry
- [x] Go client library (`pkg/client`)
- [x] RAG demo with mock + live modes
- [x] MCP server for Claude Desktop
- [x] HNSW graph index — O(log N) ANN search (v2.0)
- [x] Binary snapshot persistence for HNSW
- [ ] Token authentication (`--auth` flag)
- [ ] Dot product similarity mode (`METHOD DOT`)
- [ ] Batch VSET (`VMSET`) for bulk ingestion
- [ ] Prometheus metrics endpoint

---

## License

MIT. Build something useful with it.

---

<div align="center">

Built to scratch an itch. Benchmarks are real. The JSON deserialization tax is real.

**If this saved you from deploying Chroma on a t3.micro, consider starring the repo.**

</div>