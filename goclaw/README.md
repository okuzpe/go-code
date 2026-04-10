# goclaw

This repository’s **only active project** is **goclaw** (this Go module). Other trees at the repo root (for example `claw-code/`) are **reference only** — not part of `go.mod`, not covered by goclaw issues or roadmap.

Go CLI coding agent — **local-first** with Ollama (`qwen2.5-coder:14b` by default), optional Anthropic API or any **OpenAI Chat Completions–compatible** endpoint (OpenRouter, Groq, LM Studio, etc.). No cloud required for the default path.

## Requirements

- **Go** `1.26+` (see `go.mod`)
- **Default stack:** [Ollama](https://ollama.com/) on `http://localhost:11434` with a model pulled (default model name in settings)
- **Anthropic (optional):** `ANTHROPIC_API_KEY` and `"provider": "anthropic"` in settings
- **OpenAI-compatible APIs (optional):** `"provider": "openai_compatible"` plus `OPENAI_BASE_URL` (include `/v1`, e.g. `https://openrouter.ai/api/v1`), `OPENAI_API_KEY`, and `OPENAI_MODEL` — or the same values as `openai_base_url`, `openai_api_key`, and `openai_model` in `settings.json` (see [CLAUDE.md](CLAUDE.md#environment-variables))

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

Details (modes, sessions, JSON output, troubleshooting): **[usage.md](../docs/goclaw/usage.md)**.

## Documentation

**Master file list and reading order:** **[docs-map.md](../docs/docs-map.md)**.

| Doc | Role |
|-----|------|
| [**documentation.md**](../docs/goclaw/documentation.md) | Where each doc belongs; naming; Diátaxis-style principles |
| [**code-adjustment-map.md**](../docs/reference/code-adjustment-map.md) | Maps `docs/` topics to `internal/*` packages when changing behavior |
| [**usage.md**](../docs/goclaw/usage.md) | Day-to-day: run modes, sessions, `prompt`/JSON, flags, slash commands, Anthropic, troubleshooting |
| [**CLAUDE.md**](CLAUDE.md) | Architecture, tool contract, env vars, packages (source of truth for agents) |
| [**roadmap.md**](../docs/goclaw/roadmap.md) | Product checklist and CI notes |
| [**philosophy.md**](../docs/goclaw/philosophy.md) | UX principles |
| [**changelog.md**](../docs/goclaw/changelog.md) | Version-to-version user-visible changes |
| [**scripts/MOCK_PARITY_HARNESS.md**](scripts/MOCK_PARITY_HARNESS.md) | Mock Anthropic regression bundle (`make parity`) |

**Cross-cutting specs (under `docs/`):** [architecture.md](../docs/architecture.md), [code-adjustment-map.md](../docs/reference/code-adjustment-map.md), [tool-contract.md](../docs/reference/tool-contract.md), [mcp.md](../docs/reference/mcp.md), [hooks.md](../docs/reference/hooks.md), [agent-profiles.md](../docs/reference/agent-profiles.md), [coordinator-mode.md](../docs/reference/coordinator-mode.md), [archive index](../docs/archive/README.md).

### Markdown folders (next to the module)

| Folder | Contents |
|--------|----------|
| [docs/reference/](../docs/reference/) | Shared contracts (tools, MCP, hooks, …) |
| [docs/goclaw/](../docs/goclaw/) | Topic files — table below |
| [docs/openclaw/](../docs/openclaw/) | Product notes (not the Go tree) |
| [docs/archive/](../docs/archive/) | Long-form / archived drafts |

### Topic files (`docs/goclaw/`)

| File | Topic |
|------|--------|
| [usage.md](../docs/goclaw/usage.md) | Run modes, sessions, config, CLI, troubleshooting |
| [documentation.md](../docs/goclaw/documentation.md) | Doc layout and principles |
| [roadmap.md](../docs/goclaw/roadmap.md) | Product checklist |
| [philosophy.md](../docs/goclaw/philosophy.md) | UX and scope |
| [changelog.md](../docs/goclaw/changelog.md) | Release history |
| [coordinator.md](../docs/goclaw/coordinator.md) | D16: `spawn_agent`, workers |
| [swarm.md](../docs/goclaw/swarm.md) | Disk mailbox hub vs coordinator |
| [mcp-remote.md](../docs/goclaw/mcp-remote.md) | MCP bearer, threats, future OAuth/WS |
| [mcp-servers.example.json](../docs/goclaw/mcp-servers.example.json) | Example `mcp_servers` entries |
| [manual-tui-checklist.md](../docs/goclaw/manual-tui-checklist.md) | Manual Bubble Tea / readline QA |
| [ollama-stack.md](../docs/goclaw/ollama-stack.md) | Optional local 7B/8B stack, `compaction_model`, Ollama multi-load |
| [i18n.md](../docs/goclaw/i18n.md) | LLM language vs English UI |
| [security.md](../docs/goclaw/security.md) | Security notes |
| [prefix-input-modes.md](../docs/goclaw/prefix-input-modes.md) | Deferred input modes |

From the **goclaw** directory, link to topic files as `../docs/goclaw/<name>.md`.

## Why goclaw

- Runs **fully local** with Ollama — no API key required for the default provider.
- **Profiles** with tool allowlists (`explore`, `plan`, `coordinator`, …) via `--profile` or `settings.json`.
- **Sessions** (JSONL) and **memory** (Markdown under `~/.goclaw/memory/`).
- **Permissions** per tool (`ask` / `allow` / `deny`) and optional hooks.

**Anthropic (optional):** set `ANTHROPIC_API_KEY`, add `"provider": "anthropic"` in `~/.goclaw/settings.json`. Model aliases: see [usage.md](../docs/goclaw/usage.md).

**OpenAI-compatible (optional):** `"provider": "openai_compatible"` with base URL, API key, and model id — full matrix and custom-agent `model` overrides: [CLAUDE.md](CLAUDE.md#environment-variables).

**Development:** `go test ./...`, `go vet ./...`, `make parity`. CI: [`.github/workflows/goclaw-ci.yml`](../.github/workflows/goclaw-ci.yml).

## Multi-agent (brief)

- **Shipped:** `--profile coordinator` uses `spawn_agent` / `stop_task`; workers have isolated sessions. See [coordinator.md](../docs/goclaw/coordinator.md) and [coordinator-mode.md](../docs/reference/coordinator-mode.md).
- **Not in scope:** Team/Swarm (tmux-style peer agents).
- **External stacks** (Discord, clawhip, etc.) are optional wrappers around the CLI — not bundled here.
