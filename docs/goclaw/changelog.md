# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) where practical.

## [Unreleased]

### Added

- **`code-review`** built-in agent profile (read-only on workspace writes via allowlist) and REPL **`/review`** slash command: runs `git diff` in the workspace, injects output for one review turn. Docs: [code-review-workflow.md](code-review-workflow.md), [verification-recipe.md](verification-recipe.md), [ide-pr-parity.md](ide-pr-parity.md). Optional skill: `goclaw/.claude/skills/code-review.md`.
- **`task_models` routing:** while `code-review` is active, the rules router uses the **`reasoning`** role for every turn (see [model-routing.md](model-routing.md)); project template **`goclaw/.goclaw/verify.example.sh`** for a canonical verify entry point.

### Changed

- **TUI:** welcome panel **Guided flows** (plan + multi-agent), **Ctrl+P** opens agent picker; footer hint after `/plan save` and `/apply-plan --preview`.
- **Slash:** `/apply-plan --preview` shows a plan excerpt without executing; `/apply-plan` still runs one execution turn. `/plan save` message points at preview-then-execute.
- **bash tool (Windows):** allow **`dir`**, **`where`**, **`type`** when using CMD fallback; clearer hint when `bash`/`sh` are missing from `PATH`.
- **Documentation:** reference and roadmap wording now distinguish **shipped** behavior from **not implemented** / **Partial** (removed stale MVP/post-MVP labels where features already exist); see [documentation.md](documentation.md) terminology note and [roadmap.md](roadmap.md).

## [1.3.0] - 2026-04-11

### Added

- **Task model routing**: optional per-turn model selection via **`task_model_router`** (`off` \| `rules` \| `llm`), **`task_models`** role→model map, and optional **`task_model_router_model`** / env **`GOCLAW_TASK_MODEL_ROUTER`**, **`GOCLAW_TASK_MODEL_ROUTER_MODEL`**; CLI **`--task-model-router`**. See [model-routing.md](model-routing.md).
- Orchestrator: **runtime user-language hints** — heuristic + **whatlanggo** when reliable + optional **`preferred_response_language`** (`auto` \| `from_os` \| `es` \| `en` \| `fr` \| `de` \| `pt`) and locale fallback (`internal/orchestrator/user_language_*.go`); see [i18n.md](i18n.md).
- Config: optional **`compaction_model`** (and **`GOCLAW_COMPACTION_MODEL`**) so LLM-driven compaction (`llm_compaction`) can call a smaller/faster model than the main turn; see [`ollama-stack.md`](ollama-stack.md).
- Project template under **`goclaw/.goclaw/`**: `settings.json` tuned for a local 7B/8B stack and custom agents `stack-base`, `stack-coder`, `stack-coordinator`, `stack-explore`.
- **Documentation:** [roadmap.md](roadmap.md) now states **shipped scope** (Tiers 0–8) vs **Future** / **Partial** gaps, plus **optional follow-up waves** A–D; reference docs updated to drop stale “MVP/post-MVP” wording where features are already implemented.

### Changed

- **Documentation:** **[docs-map.md](../docs-map.md)** remains the master index (audience, reading order, **language policy** note, **Doc maintenance** log, corrected **prefix-input-modes** row); **`docs/reference/`** link text aligned to kebab-case filenames and stale “ARCHITECTURE §” callouts replaced with **CLAUDE.md** / **roadmap**; **[README.md](../../goclaw/README.md)** defers long lists to docs-map; **[architecture.md](../architecture.md)** hub shortened; per-file doc changelogs merged into docs-map. OpenClaw-derived ideas live in **[philosophy.md](philosophy.md#lessons-from-wider-agent-stacks)** and **[roadmap.md](roadmap.md#future-transport-and-scale)**; local OpenClaw markdown removed; **`docs/archive/`** removed (use git history if needed).
- **Default `agent_profile`** is now **`general-purpose`** (direct file/bash tools on the main session). Hub-and-spoke remains available via **`coordinator`** (`/profile coordinator`, `agent_profile`, `GOCLAW_AGENT_PROFILE`, or `make run-hub`). See [usage.md — Agent profiles](usage.md#agent-profiles) and [coordinator.md](coordinator.md).
- Base system prompt (`internal/orchestrator/base_system_prompt.md`): **RESPONSE LANGUAGE** block at the top—English instructions do not imply English replies; short greetings must match user language; see [i18n.md](i18n.md).

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
