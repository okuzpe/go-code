# goclaw

This repository's **only active project** is **goclaw** (this Go module). Other trees at the repo root are reference-only and not part of `go.mod`.

Go CLI coding agent, **local-first** with Ollama (`qwen2.5-coder:7b` by default via `config.DefaultOllamaModel`; override with `ollama_model` / `OLLAMA_MODEL`). The shipped CLI talks to your Ollama daemon only; no bundled cloud LLM providers.

## Requirements

- **Go** `1.26+` (see `go.mod`)
- **Default stack:** [Ollama](https://ollama.com/) on `http://localhost:11434` with a model pulled

## Golden Path

```bash
cd goclaw
go run ./cmd/goclaw doctor    # health check
go run ./cmd/goclaw           # fullscreen TUI on a TTY
printf 'ping\n' | go run ./cmd/goclaw --mock --no-tools --output-format json
```

Inside the TUI, the primary flow is:

- **`build`** for direct coding
- **`plan`** for read-only planning first

On a TTY, the first interactive launch runs first-time setup until `~/.goclaw/settings.json` exists (see [usage.md - First-run setup](../docs/goclaw/usage.md#first-run-setup-onboarding)).

Install a binary from this checkout:

```bash
go build -o goclaw ./cmd/goclaw
```

Details (modes, sessions, JSON output, troubleshooting): **[usage.md](../docs/goclaw/usage.md)**. If the assistant explains but never edits files: **[usage.md - troubleshooting](../docs/goclaw/usage.md#assistant-explains-plans-but-does-not-modify-files)**.

## Why goclaw

- Runs **fully local** with Ollama - no API key required for the default provider.
- Keeps the primary workflow simple: **`build`** and **`plan`**.
- Supports persisted **sessions** (JSONL) and **memory** (Markdown under `~/.goclaw/memory/`).
- Enforces **permissions** per tool (`ask` / `allow` / `deny`) with optional hooks.

## Advanced

- **Advanced profiles** stay available through `--profile` or `/profile` when you explicitly need them (`builder`, `coordinator`, `verification`, `code-review`, and custom agents).
- **Coordinator mode** is optional hub-and-worker delegation via `spawn_agent`; keep the normal local loop on `build` unless you specifically want delegation.
- **Optional integrations** such as Telegram, plugins, skills, and MCP remote stay supported, but live outside the default day-to-day path.

## Documentation

**Master index** (every path, audience, reading order, `docs/reference/` contracts): **[docs-map.md](../docs/docs-map.md)**.

**Core links:** [CLAUDE.md](CLAUDE.md) · [usage.md](../docs/goclaw/usage.md) · [documentation.md](../docs/goclaw/documentation.md) · [code-adjustment-map.md](../docs/reference/code-adjustment-map.md).

**Development:** `go test ./...`, `go vet ./...`, `make parity`. CI: [`.github/workflows/goclaw-ci.yml`](../.github/workflows/goclaw-ci.yml).
