# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) where practical.

## [Unreleased]

### Added

- **Task model routing**: optional per-turn model selection via **`task_model_router`** (`off` \| `rules` \| `llm`), **`task_models`** role→model map, and optional **`task_model_router_model`** / env **`GOCLAW_TASK_MODEL_ROUTER`**, **`GOCLAW_TASK_MODEL_ROUTER_MODEL`**; CLI **`--task-model-router`**. See [model-routing.md](model-routing.md).
- Orchestrator: **runtime user-language hints** — heuristic + **whatlanggo** when reliable + optional **`preferred_response_language`** (`auto` \| `from_os` \| `es` \| `en` \| `fr` \| `de` \| `pt`) and locale fallback (`internal/orchestrator/user_language_*.go`); see [i18n.md](i18n.md).
- Config: optional **`compaction_model`** (and **`GOCLAW_COMPACTION_MODEL`**) so LLM-driven compaction (`llm_compaction`) can call a smaller/faster model than the main turn; see [`ollama-stack.md`](ollama-stack.md).
- Project template under **`goclaw/.goclaw/`**: `settings.json` tuned for a local 7B/8B stack and custom agents `stack-base`, `stack-coder`, `stack-coordinator`, `stack-explore`.

### Changed

- Base system prompt (`internal/orchestrator/base_system_prompt.md`): **RESPONSE LANGUAGE** block at the top—English instructions do not imply English replies; short greetings must match user language; see [i18n.md](i18n.md).
- Documentation layout: goclaw topic files live under monorepo **`docs/goclaw/`** with **kebab-case** names (`coordinator.md`, `mcp-remote.md`, `swarm.md`, `manual-tui-checklist.md`). Cross-cutting specs: **`docs/reference/`**; OpenClaw notes: **`docs/openclaw/`**; Spanish long-form: **`docs/archive/architecture-legacy-es.md`**; English hub: [architecture.md](../architecture.md).
- Operator / product docs moved to **`docs/goclaw/`**: [usage.md](usage.md), [documentation.md](documentation.md), [roadmap.md](roadmap.md), [philosophy.md](philosophy.md), [changelog.md](changelog.md). **`goclaw/`** keeps [README.md](../../goclaw/README.md) + [CLAUDE.md](../../goclaw/CLAUDE.md). Master index: [docs-map.md](../docs-map.md). No `docs/README.md` (removed); repo root [README.md](../../README.md) points here.

## [1.2.0] - 2026-04-09

### Added

- Coordinator `stop_task` tool to cancel in-flight `spawn_agent` workers via `task_id` from spawn results.
- Session list metadata: `/sessions` shows RFC3339 modification time per saved session.
- Memory list previews: first 80 runes of entry body in `/memory list`.
- `/compact` confirmation includes message counts before and after compaction.
- Non-blocking Ollama reachability warning at startup (`internal/app/ollama_probe.go`).
- TUI footer shows elapsed seconds while a tool is running.
- Tests for hooks (HTTP JSON body, stdin JSON for commands, exit 1), MCP registration naming, read-only MCP execution, MCP start failure tolerance, trusted project hooks, race workflow notes, and manual TUI checklist doc.
- GoReleaser configuration for cross-platform archives (`.goreleaser.yaml`).
- MCP `Conn` interface (`internal/mcp/conn.go`) — clean abstraction over stdio and HTTP transports.
- MCP Streamable HTTP client (`internal/mcp/http.go`): `HTTPSession` implements `Conn` for `mcp_servers[].url`; loopback-only by default, opt-in remote via `mcp_allow_remote_urls`; SSE + JSON response modes.
- `ResilientConn` (`internal/mcp/resilient.go`): one-shot reconnect on recoverable transport errors (EOF, broken pipe, reset); transparent to callers.
- IDE lockfile discovery (`internal/ide/discovery.go`): reads `~/.goclaw/ide/*.json` for MCP HTTP endpoints; used when `ide_bridge_mcp` is set.
- `internal/text` package: `TruncateRunes` for rune-safe string truncation (extracted from `ui/chat`).
- Config: `mcp_allow_remote_urls` setting to permit non-loopback MCP HTTP endpoints (default off).
- Config: `ide_bridge_mcp` setting to auto-discover editor MCP endpoint from `~/.goclaw/ide/*.json` lockfiles.
- Config: `MCPServerConfig` now accepts `url` and `headers` fields for Streamable HTTP servers.
- `doctor` / `/doctor` now shows HTTP MCP connection hints (SSRF policy, HTTPS loopback certificate, unauthorized errors).

### Changed

- `spawn_agent` JSON results include `task_id` for use with `stop_task`.
- Clearer MCP `tools/call` errors when the server connection drops mid-request.
- `mcp.ToolAdapter` and `RegisterSessionTools` accept the `mcp.Conn` interface instead of `*mcp.Session`, enabling HTTP and stdio transports interchangeably.
- `chat.go`: `truncateRunes` extracted to `internal/text.TruncateRunes`; local copy removed.
