# goclaw

This repository’s **only active project** is **goclaw** (this Go module). Other trees at the repo root (for example `claw-code/`) are **reference only** — not part of `go.mod`, not covered by goclaw issues or roadmap.

Go CLI coding agent — **local-first** with Ollama (`qwen2.5-coder:14b` by default via `config.DefaultOllamaModel`; use `qwen2.5-coder:7b` in settings if VRAM is tight). The CLI talks only to your Ollama daemon; no bundled cloud LLM providers.

## Requirements

- **Go** `1.26+` (see `go.mod`)
- **Default stack:** [Ollama](https://ollama.com/) on `http://localhost:11434` with a model pulled (default model name in settings)

## Quick start

```bash
cd goclaw
go run ./cmd/goclaw doctor    # health check
go run ./cmd/goclaw           # fullscreen TUI on a TTY (default)
go run ./cmd/goclaw --readline # line-at-a-time REPL
```

On a TTY, the first interactive launch runs first-time setup until `~/.goclaw/settings.json` exists (see [usage.md — First-run setup](../docs/goclaw/usage.md#first-run-setup-onboarding)).

Install a binary from this checkout:

```bash
go build -o goclaw ./cmd/goclaw
```

Details (modes, sessions, JSON output, troubleshooting): **[usage.md](../docs/goclaw/usage.md)**. If the assistant **explains but never edits files**: **[usage.md — troubleshooting](../docs/goclaw/usage.md#assistant-explains-plans-but-does-not-modify-files)**. Whole-repo audits: **[Large repo analysis and refactors](../docs/goclaw/usage.md#large-repo-analysis-and-refactors)**.

## Documentation

**Master index** (every path, audience, reading order, `docs/reference/` contracts): **[docs-map.md](../docs/docs-map.md)**.

**Core links:** [CLAUDE.md](CLAUDE.md) (implementation) · [usage.md](../docs/goclaw/usage.md) (operators) · [ide-editor-setup.md](../docs/goclaw/ide-editor-setup.md) (editor MCP lockfile golden path) · [documentation.md](../docs/goclaw/documentation.md) (where to add docs) · [code-adjustment-map.md](../docs/reference/code-adjustment-map.md) (docs ↔ `internal/*`). **Mock LLM harness:** [scripts/MOCK_PARITY_HARNESS.md](scripts/MOCK_PARITY_HARNESS.md) (`make parity`).

Everything else (roadmap, changelog, architecture hub, tool/MCP/hooks reference): use the tables in **docs-map.md** — from this directory, topic files live under `../docs/goclaw/<name>.md` or `../docs/reference/<name>.md`.

## Why goclaw

- Runs **fully local** with Ollama — no API key required for the default provider.
- **Profiles** with tool allowlists (`explore`, `plan`, `coordinator`, …) via `--profile` or `settings.json`.
- **Sessions** (JSONL) and **memory** (Markdown under `~/.goclaw/memory/`).
- **Permissions** per tool (`ask` / `allow` / `deny`) and optional hooks.

**Development:** `go test ./...`, `go vet ./...`, `make parity`. CI: [`.github/workflows/goclaw-ci.yml`](../.github/workflows/goclaw-ci.yml).

## Multi-agent (brief)

- **Default session:** `general-purpose` (file/bash tools on the main agent). **`make run-hub`** or **`--profile coordinator`** for hub-and-spoke (`spawn_agent` / `stop_task`; workers have isolated sessions). See [coordinator.md](../docs/goclaw/coordinator.md) and [coordinator-mode.md](../docs/reference/coordinator-mode.md).
- **Not in scope:** Team/Swarm (tmux-style peer agents).
- **External stacks** (Discord, clawhip, etc.) are optional wrappers around the CLI — not bundled here.
