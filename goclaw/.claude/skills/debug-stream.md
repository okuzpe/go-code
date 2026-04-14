---
name: debug-stream
description: Use when the LLM misbehaves, streaming looks wrong, or tool calls are not applied correctly.
---

## Debug: streaming and tool calls

### Checklist

#### 1. Is Ollama running?
```bash
curl http://localhost:11434/api/tags
# expect JSON model list; if not: ollama serve
```

#### 2. Is the model pulled?
```bash
ollama list
ollama pull qwen2.5-coder:14b   # or your configured model (e.g. qwen2.5-coder:7b on low VRAM)
```

#### 3. Does bare `/api/chat` work?
```bash
curl http://localhost:11434/api/chat -d '{
  "model": "qwen2.5-coder:14b",
  "messages": [{"role": "user", "content": "ping"}],
  "stream": false
}'
```

#### 4. Go client connectivity
Temporarily log in `internal/llm/ollama.go` before the read loop, e.g. model and host. Run with `GOCLAW_LOG=debug`.

#### 5. Chunks empty on the last line?
Streaming NDJSON often sends content in incremental `message` fields; goclaw buffers tool/native content until `done`. If you changed that logic, compare with upstream.

#### 6. Tool calls on Ollama
Ollama `/api/chat` supports tools. goclaw:
- Parses `message.tool_calls` from streamed chunks (and a JSON-in-content fallback for some models).
- **Critical**: tool result messages must use JSON field **`tool_name`**, not `name`, or the next turn will ignore results (see `ollama_wire.go` and Ollama docs).
- Not every local model supports tool calling reliably; try a known tool-capable tag.

#### 7. `iteration limit (32) reached`
Session too long without a final assistant text, or model stuck in tools. Compaction reduces bulk; start a new session if needed.

### Useful env
```bash
OLLAMA_HOST=http://127.0.0.1:11434 OLLAMA_MODEL=qwen2.5-coder:7b go run ./cmd/goclaw
```

### Key files
- `internal/llm/ollama.go`, `ollama_wire.go`
- `internal/orchestrator/orchestrator.go`
