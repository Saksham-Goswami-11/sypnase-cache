# Synapse Cache: Local-First Agentic RAG Server

Synapse Cache is a high-performance, in-memory **Vector Database** written from scratch in Go, paired with a fully functional Python-based **Hybrid Retrieval-Augmented Generation (RAG)** pipeline. The system is designed to expose a folder of local documents as an intelligent tool for AI agents via the **Model Context Protocol (MCP)**.

It uses a custom raw TCP wire protocol (similar to Redis RESP) to handle lightning-fast semantic searches (`VSIMILARITY`), native vector storage, and chunk management.

---

## 🏗️ Architecture & Features

### Core Stack
*   **Database Engine**: Custom Go in-memory vector database (`synapse-cache`).
*   **Backend Client**: Python 3.11+ using raw TCP sockets.
*   **Embeddings**: OpenAI `text-embedding-3-small` (1536 dimensions) or deterministic local mocks.
*   **Agent Protocol**: Official Python MCP SDK (`mcp.server.fastmcp`).
*   **Deployment**: Fully containerized with Docker Compose.

### Key Features
1.  **Custom TCP Vector DB**: Built entirely from scratch in Go. Includes support for custom commands like `VSET`, `VGET`, and `VSIMILARITY`.
2.  **Hybrid RAG Retrieval**: Uses **Reciprocal Rank Fusion (RRF)** to combine exact keyword matches (via `BM25`) with semantic vector matching (cosine similarity) for highly accurate document retrieval.
3.  **Watchdog Ingestion**: A daemon that continuously monitors a `./knowledge_base` folder. Drop a `.md` or `.pdf` file in, and it instantly chunks, embeds, and loads it into the Go DB.
4.  **Robust Protocol & Security**: 
    *   **TLS 1.2+ Encryption**: All raw socket communications are fully encrypted.
    *   **Password Authentication**: Hardened constant-time `AUTH` protocol mechanism prevents unauthorized queries.
    *   **LFI & OOM Protection**: Strict path-traversal jails for file ingestion and bounded-buffer allocation strategies protect against Local File Inclusion and Out-of-Memory Denial of Service attacks.
5.  **MCP Integration**: Native integration with Claude Desktop via the `search_internal_knowledge` tool.

---

## 🚀 Quick Start Guide (Docker)

The fastest way to get Synapse Cache up and running is via Docker.

### 1. Configure the Environment
Create a `.env` file in the root of the project to securely inject your passwords and API keys:

```env
# Database Credentials
SYNAPSE_PASSWORD="YourSuperSecurePassword123!"

# OpenAI Integration
OPENAI_API_KEY="sk-your-actual-openai-api-key-here"

# Security Settings
SYNAPSE_TLS="true"
SYNAPSE_INSECURE_SKIP_VERIFY="true"
```

### 2. Start the Cluster
Launch the full stack (Database, Ingestion Daemon, and MCP Server) in detached mode:
```bash
docker compose up --build -d
```

### 3. Add Knowledge
Simply drag and drop any `.md` or `.pdf` file into the `./knowledge_base` folder. The Python watchdog will instantly detect it, parse it, chunk it, request vector embeddings, and securely store it into the Go database!

---

## 💻 Manual Developer Setup

If you wish to run the stack natively without Docker:

### Prerequisites
*   **Go** (1.21+)
*   **Python** (3.11+)

### 1. Build and Run the Go Database Server
Build the core Synapse Cache server and start it with TLS and Password Authentication enabled:
```bash
go build -o bin/synapse-server ./cmd/server
go build -o bin/synapse-cli ./cmd/cli

# Generate local developer certificates
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 3650 -nodes -subj "/CN=localhost"

# Start the server on port 6380
./bin/synapse-server -port 6380 -aof ./synapse.aof -tls -cert ./cert.pem -key ./key.pem -password "YourSuperSecurePassword123!"
```

### 2. Setup Python Environment & Run Daemon
Initialize the Python virtual environment and run the watchdog:
```bash
python3 -m venv mcp_server/.venv
source mcp_server/.venv/bin/activate
pip install -r mcp_server/requirements.txt

# Run the ingestion daemon manually
export SYNAPSE_PASSWORD="YourSuperSecurePassword123!"
export SYNAPSE_TLS="true"
export SYNAPSE_INSECURE_SKIP_VERIFY="true"
python3 mcp_server/ingest.py
```

### 3. Run Integration Tests
To verify the entire pipeline over the encrypted TCP protocol:
```bash
source mcp_server/.venv/bin/activate
python3 mcp_server/test_mcp.py
```

---

## 🛠️ Using the Interactive CLI

You can manually interact with your custom Go database using the built-in CLI REPL:
```bash
./bin/synapse-cli -addr localhost:6380
```
*(Note: If TLS is enabled, ensure your CLI client supports TLS wrapping).*

**Common Commands:**
*   `AUTH <password>`: Authenticate with the database.
*   `SET <key> "<value>"`: Store a text payload (supports escaped newlines `\n`).
*   `GET <key>`: Retrieve a payload.
*   `VSET <namespace> <id> <dim> <f1> <f2>... META <key> <val>`: Store a vector with metadata.
*   `VSIMILARITY <namespace> <dim> <f1> <f2>... TOP <k>`: Search for similar vectors.

---

## 🤖 Connecting to Claude Desktop via MCP

You can expose the Hybrid RAG engine directly to Claude Desktop, allowing the AI to search your local files natively.

1. Open your Claude Desktop Configuration file:
   * **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
   * **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
2. Add the `synapse-rag` server configuration:

```json
{
  "mcpServers": {
    "synapse-rag": {
      "command": "/Users/sakshamgoswami/Documents/sypnase-cache/mcp_server/.venv/bin/python3",
      "args": ["/Users/sakshamgoswami/Documents/sypnase-cache/mcp_server/server.py"],
      "env": {
        "OPENAI_API_KEY": "sk-your-openai-api-key",
        "KNOWLEDGE_BASE_DIR": "/Users/sakshamgoswami/Documents/sypnase-cache/knowledge_base",
        "SYNAPSE_PORT": "6380",
        "SYNAPSE_PASSWORD": "YourSuperSecurePassword123!",
        "SYNAPSE_TLS": "true",
        "SYNAPSE_INSECURE_SKIP_VERIFY": "true"
      }
    }
  }
}
```
*(Make sure to update the absolute paths if your repository is located elsewhere).*

3. **Restart Claude Desktop**. You will now see the `search_internal_knowledge` tool (🛠️) available. Ask Claude questions like:
   * *"What are the company working hours based on my local documents?"*
   * *"Search my internal knowledge base for the Synapse Cache database key."*

---

## 🧪 Testing and Contributing

We welcome contributions to Synapse Cache! Here is how you can help:

### Running Tests
To verify the core Go database tokenizers, similarity calculations, and storage engines:
```bash
go test -race -v ./...
```

### Contribution Guidelines
1. **Fork and Clone**: Fork the repository and create a new feature branch (`git checkout -b feature/awesome-feature`).
2. **Security First**: Synapse Cache acts as an infrastructure layer. If you add new endpoints or parsing logic, ensure you strictly enforce bounded-buffer limits to prevent Memory Exhaustion (OOM) vulnerabilities.
3. **No External Vector DBs**: The goal of this project is to implement the underlying mathematics (like Cosine Similarity) from scratch in Go. Do not pull in large external vector database SDKs like Pinecone or ChromaDB.
4. **Format & Lint**: Ensure all Go code is properly formatted (`go fmt ./...`) and Python code adheres to PEP-8 standards.
5. **Submit a PR**: Open a Pull Request with a clear description of the problem you are solving and the verification steps you took.

## License
MIT License
