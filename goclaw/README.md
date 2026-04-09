# goclaw

Go CLI coding agent — **local-first** with Ollama (`qwen2.5-coder:14b` by default), optional Anthropic API. No cloud required for the default path.

## Documentation

| Doc | Audience |
|-----|----------|
| [**USAGE.md**](USAGE.md) | Day-to-day: run modes, sessions, `prompt`/JSON, CLI flags, slash commands, Anthropic setup, troubleshooting |
| [**CLAUDE.md**](CLAUDE.md) | Architecture, tool contract, env vars, packages, conventions (source of truth for agents) |
| [**ROADMAP.md**](ROADMAP.md) | Product checklist and CI notes |
| [**PHILOSOPHY.md**](PHILOSOPHY.md) | UX principles |
| [**docs/D16_COORDINATOR_SKETCH.md**](docs/D16_COORDINATOR_SKETCH.md) | Coordinator profile, `spawn_agent`, worker isolation |
| [**docs/mcp_servers.example.json**](docs/mcp_servers.example.json) | Template `mcp_servers` entries (merge into `settings.json`; see repo [MCP.md](../MCP.md)) |
| [**scripts/MOCK_PARITY_HARNESS.md**](scripts/MOCK_PARITY_HARNESS.md) | Mock Anthropic regression bundle (`make parity`) |

**Monorepo root** (parent of `goclaw/`): [TOOL_CONTRACT.md](../TOOL_CONTRACT.md), [MCP.md](../MCP.md), [HOOKS.md](../HOOKS.md), [AGENT_PROFILES.md](../AGENT_PROFILES.md), [COORDINATOR_MODE.md](../COORDINATOR_MODE.md), [DOCS_MAP.md](../DOCS_MAP.md).

## Why goclaw

- Runs **fully local** with Ollama — no API key required for the default provider.
- **Profiles** with tool allowlists (`explore`, `plan`, `coordinator`, …) via `--profile` or `settings.json`.
- **Sessions** (JSONL) and **memory** (Markdown under `~/.goclaw/memory/`).
- **Permissions** per tool (`ask` / `allow` / `deny`) and optional hooks.

## Quick start

```bash
cd goclaw
go run ./cmd/goclaw doctor    # health check
go run ./cmd/goclaw           # readline REPL on TTY (default)
go run ./cmd/goclaw --tui     # fullscreen UI
```

Build a binary:

```bash
go build -o goclaw ./cmd/goclaw
```

**Anthropic (optional):** set `ANTHROPIC_API_KEY`, add `"provider": "anthropic"` in `~/.goclaw/settings.json`. Model: `GOCLAW_MODEL` (short aliases `opus`, `sonnet`, `haiku` — see [USAGE.md](USAGE.md)).

**Development:** `go test ./...`, `go vet ./...`, `make parity` (mock harness). CI: [`.github/workflows/goclaw-ci.yml`](../.github/workflows/goclaw-ci.yml).

## Multi-agent (brief)

- **Shipped:** `--profile coordinator` uses `spawn_agent` / `stop_task`; workers have isolated sessions. See [docs/D16_COORDINATOR_SKETCH.md](docs/D16_COORDINATOR_SKETCH.md) and [COORDINATOR_MODE.md](../COORDINATOR_MODE.md).
- **Not in scope:** Team/Swarm (tmux-style peer agents).
- **External stacks** (Discord, clawhip, etc.) are optional wrappers around the CLI — not bundled here.
