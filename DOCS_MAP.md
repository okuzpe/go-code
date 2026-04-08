# Documentation Map

Single entry point for humans and AI agents: which file covers which topic, and whether it describes implemented goclaw behavior or a future design.

**Source of truth for the goclaw binary:** [`goclaw/CLAUDE.md`](goclaw/CLAUDE.md) — architecture decisions D0–D22, coding conventions, roadmap.

---

## goclaw Implementation Status

| Area | Status |
|------|--------|
| Entry point | Thin `goclaw/cmd/goclaw` (`main.go` + `version.go`): slog + [`internal/cli`](goclaw/internal/cli/root.go) Cobra tree + [`internal/app/run.go`](goclaw/internal/app/run.go); **default interactive UI** = readline `>` REPL (claw-style); **`--tui`** / `GOCLAW_USE_TUI=1` for Bubble Tea; flags `--profile`/`--session`/`--list-sessions`/`--no-tools`; slash commands in [`internal/slashcmd`](goclaw/internal/slashcmd/slash.go) |
| Packages | `internal/llm`, `orchestrator`, `session`, `tools`, `permissions`, `config`, `hooks`, `agents`, `memory`, `planfile`, `todos`, `mcp`, `ide`, `ui/chat` (BubbleTea TUI) |
| Tools | Nine builtins: `read_file`, `write_file`, `edit_file`, `glob`, `grep`, `bash`, `web_fetch`, `web_search`, `todo_write`; MCP tools as `mcp__<id>__<name>` |
| Plan workflow | Workspace `.goclaw/plan.md` ([`internal/planfile`](goclaw/internal/planfile/planfile.go)); `/apply-plan` switches to `general-purpose` and runs one execution turn |
| Memory | `~/.goclaw/memory/` + `MEMORY.md` index; REPL `/memory list|add|delete` |
| Compaction | Token-estimate heuristic (char/4), 0.85 threshold, 24-turn tail preserved |
| Hooks | Same five events; Go `hooks.Registry`, **`external_hooks`** (subprocess stdin JSON or HTTP POST in settings), and project **`.goclaw/hooks.json`** when `trusted_workspace` is true ([`internal/hooks`](goclaw/internal/hooks)) |
| MCP | **stdio client** only — `mcp_servers` in merged settings, JSON-RPC in [`internal/mcp`](goclaw/internal/mcp); multi-server with per-server failure isolation |
| IDE | **Partial** — best-effort POST to `GOCLAW_IDE_NOTIFY_URL` (localhost-only URL validation); not a full editor MCP client ([`internal/ide`](goclaw/internal/ide/notify.go)) |
| Retries | `internal/llm/retry.go` — 10 attempts, 500 ms→5 min exp backoff, 429/503/504 (D22) |
| Profiles | 6 built-in in `internal/agents/profile.go` |
| Not in goclaw (yet) | Plugins, **remote** MCP (SSE/HTTP/WebSocket) + OAuth, full IDE bridge (lockfile MCP), multi-agent coordinator (D16), YOLO classifier (D17), Markdown-defined custom agents (D19) |

---

## Reading Order (for implementation work)

1. [`goclaw/CLAUDE.md`](goclaw/CLAUDE.md) — real module state, decisions D1–D22, conventions
2. [`ARCHITECTURE.md`](ARCHITECTURE.md) §1 — product scope
3. [`ARCHITECTURE.md`](ARCHITECTURE.md) §3.1 — orchestrator loop
4. [`ARCHITECTURE.md`](ARCHITECTURE.md) §4.4 — documentation phases and goclaw note
5. [`ARCHITECTURE.md`](ARCHITECTURE.md) §5 — D1–D5, D22
6. [`TOOL_CONTRACT.md`](TOOL_CONTRACT.md) — tool limits, network policy, loop budget
7. Deep dives: [`RETRY_LOGIC.md`](RETRY_LOGIC.md), [`HOOKS.md`](HOOKS.md), [`MCP.md`](MCP.md) (opening section = goclaw stdio), [`AGENT_PROFILES.md`](AGENT_PROFILES.md)

---

## File Index

| File | Topic | goclaw |
|------|-------|--------|
| [`goclaw/README.md`](goclaw/README.md) | Quick start, features, config, CLI flags, slash commands, plan file workflow | Implemented |
| [`goclaw/docs/D16_COORDINATOR_SKETCH.md`](goclaw/docs/D16_COORDINATOR_SKETCH.md) | D16 hub-and-spoke coordinator — WorkerNotification sketch (pre-code) | Design |
| [`goclaw/CLAUDE.md`](goclaw/CLAUDE.md) | Rules, D1–D22 condensed, package layout, conventions, roadmap | Source of truth |
| [`AGENT_PROFILES.md`](AGENT_PROFILES.md) | 6 built-in profiles — tool filtering, system prompts, v2+ roadmap | Implemented |
| [`HOOKS.md`](HOOKS.md) | Hook event system — 5 events, Go handlers + `external_hooks` + `.goclaw/hooks.json` | Implemented |
| [`MCP.md`](MCP.md) | MCP naming, transports, auth (reference); **Implemented in goclaw** = stdio client + `mcp_servers` | Partial (stdio implemented; SSE/HTTP/OAuth future) |
| [`IDE_BRIDGE.md`](IDE_BRIDGE.md) | Full IDE integration design | Design (notifier only in goclaw) |
| [`RETRY_LOGIC.md`](RETRY_LOGIC.md) | HTTP retry behavior — parameters, conditions, per-call budget | Implemented |
| [`TOOL_CONTRACT.md`](TOOL_CONTRACT.md) | Tool output limits, SSRF network policy, loop budgets | Implemented |
| [`ARCHITECTURE.md`](ARCHITECTURE.md) | Full design specification — D0–D22, all layers | Reference (large; not fully polished) |

---

## Post-MVP Documents

The following files exist in this directory as design specifications for future versions. They are **not** accurate descriptions of current goclaw behavior — do not reference them as implemented features.

| File | Feature | Decision | Phase |
|------|---------|----------|-------|
| `CUSTOM_AGENTS.md` | Markdown-defined custom agents | D19 | v2+ |
| `COORDINATOR_MODE.md` | Multi-agent hub-and-spoke coordinator | D16 | v2+ |
| `YOLO_CLASSIFIER.md` | Auto-mode safety classifier (bypass-permissions gate) | D17 | v2+ |
| `MCP.md` | Remote MCP transports, OAuth, enterprise policy (reference); goclaw implements **stdio only** — see “Implemented in goclaw” | D6 | v3+ (extras) |
| `PLUGINS.md` | Plugin system | D20 | v3+ |
| `IDE_BRIDGE.md` | IDE integration (VS Code, JetBrains) | D21 | v2+ |
| `LOCAL_MODELS.md` | Local model selection guide (Ollama, LM Studio) | — | Reference |
| `MEMORY_SYSTEM.md` | Detailed memory subsystem spec | — | Reference |
| `CONTEXT_COMPACTION.md` | Compaction algorithm detail | — | Reference |
| `SKILLS.md` | Skill/prompt-template system | — | Reference |

---

## External References

Full link list: [`References.MD`](References.MD).

Conceptual source used during design: [claude-code-explain (helmcode)](https://claude-code-explain.helmcode.com/) — a third-party analysis of Claude Code internals. The documents in this directory were informed by that analysis; goclaw implements a Go-native subset.

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: helmcode coverage table 00–21, MVP reading order, links. |
| 2026-04-07 | Cross-links: PRACTICAL_TIPS, OPENCLAW_AGENTS_AND_TOOLS; §2.6 ARCHITECTURE system-prompt anchor. |
| 2026-04-07 | Header: pointer to global inventory §8.0 in ARCHITECTURE. |
| 2026-04-07 | MVP reading step 5 → TOOL_CONTRACT; Tools row links contract. |
| 2026-04-07 | OpenClaw section: no local clone; GitHub + claw-code/ note. |
| 2026-04-08 | Aligned to goclaw: source of truth → goclaw/CLAUDE.md; status summary; goclaw column; reading order updated; Memory/Compaction/Hooks/Retry/Tools/Slash/Agents corrected. |
| 2026-04-08 | Translated to English; replaced 21-row helmcode table with focused File Index; added Post-MVP Documents section; OpenClaw section removed. |
| 2026-04-08 | Status table: hooks external/MCP stdio/IDE partial; nine tools; `ui/chat`; File Index HOOKS/MCP/IDE; Post-MVP MCP row clarified. |
