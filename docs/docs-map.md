# Documentation Map

Single entry point for humans and AI agents: which file covers which topic, who it is for, and how it relates to shipped goclaw behavior.

**Source of truth for the goclaw binary:** [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) — architecture decisions D1–D22, coding conventions, roadmap.

**Project README (only):** [`goclaw/README.md`](../goclaw/README.md) — requirements, quick start, short links into this map. Repo root [`README.md`](../README.md) redirects there. **Where to add new docs:** [`docs/goclaw/documentation.md`](./goclaw/documentation.md). **This file (`docs-map.md`)** is the master file index (there is no `docs/README.md`). **Languages:** `docs/goclaw/` = English product docs; `docs/reference/` = mostly Spanish design reference (see [documentation.md](./goclaw/documentation.md)).

---

## goclaw implementation status

| Area | Status |
|------|--------|
| Entry point | [`cmd/goclaw`](../goclaw/cmd/goclaw/main.go) → [`internal/cli`](../goclaw/internal/cli/root.go) (Cobra) → [`internal/app`](../goclaw/internal/app/run.go). **UI:** Bubble Tea TUI on TTY by default — flags and env in [`usage.md`](./goclaw/usage.md). Slash commands: [`internal/slashcmd`](../goclaw/internal/slashcmd/slash.go). |
| Packages | `internal/llm`, `orchestrator`, `session`, `tools`, `permissions`, `config`, `hooks`, `agents`, `memory`, `planfile`, `todos`, `mcp`, `ide`, `plugin`, `skills`, `swarm`, `ui/chat` (BubbleTea TUI) |
| Tools | Ten built-ins: `read_file`, `glob`, `grep`, `bash`, `write_file`, `edit_file`, `patch`, `web_fetch`, `web_search`, `todo_write`; optional `script` when `allow_script`; coordinator-only `spawn_agent`, `stop_task`; MCP tools as `mcp__<id>__<name>` |
| Plan workflow | Workspace `.goclaw/plan.md` ([`internal/planfile`](../goclaw/internal/planfile/planfile.go)); `/apply-plan` switches to `general-purpose` and runs one execution turn |
| Memory | `~/.goclaw/memory/` + `MEMORY.md` index; REPL `/memory list|add|delete`; opt-in auto-capture after `write_file`/`edit_file` (`memory_auto_extract`) |
| Compaction | Token-estimate heuristic (char/4), 0.85 threshold, 24-turn tail preserved; optional **`compaction_model`** + **`llm_compaction`** for LLM summaries ([`internal/orchestrator/compaction.go`](../goclaw/internal/orchestrator/compaction.go)) |
| Hooks | Same five events; Go `hooks.Registry`, **`external_hooks`** (subprocess stdin JSON or HTTP POST in settings), and project **`.goclaw/hooks.json`** when `trusted_workspace` is true ([`internal/hooks`](../goclaw/internal/hooks)) |
| MCP | stdio + streamable HTTP client — `mcp_servers` in merged settings ([`internal/mcp`](../goclaw/internal/mcp)); optional `bearer_token_file` per HTTP server; multi-server with per-server failure isolation |
| IDE | **Partial** — lockfile MCP + best-effort POST to `GOCLAW_IDE_NOTIFY_URL` (localhost-only); extension contract §7 [ide-bridge.md](./reference/ide-bridge.md) |
| Retries | `internal/llm/retry.go` — 10 attempts, 500 ms→5 min exp backoff, 429/503/504 (D22) |
| Profiles | 7 built-in in `internal/agents/profile.go` (includes `coordinator`) |
| V3 slice (partial vs full product docs) | Local plugins ([`internal/plugin`](../goclaw/internal/plugin)), SKILL.md runtime ([`internal/skills`](../goclaw/internal/skills)), swarm disk hub ([`internal/swarm`](../goclaw/internal/swarm)); **not** MCP OAuth/WS, remote marketplace, or full IDE UI |

---

## Reading order (implementation work)

1. [`docs/goclaw/usage.md`](./goclaw/usage.md) — run and configure the CLI
2. [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) — module state, decisions D1–D22, conventions
3. [`architecture.md`](./architecture.md) — short English hub (navigation only)
4. [`docs/goclaw/documentation.md`](./goclaw/documentation.md) — **optional** — where to place new Markdown (contributors)
5. [`code-adjustment-map.md`](./reference/code-adjustment-map.md) — which `docs/` topics map to which `internal/*` packages
6. [`tool-contract.md`](./reference/tool-contract.md) — tool limits, network policy, loop budget
7. [`tool-flows.md`](./reference/tool-flows.md) — Mermaid: orchestrator loop, permissions, tools, coordinator, hooks
8. Deep dives: [`retry-logic.md`](./reference/retry-logic.md), [`hooks.md`](./reference/hooks.md), [`mcp.md`](./reference/mcp.md), [`agent-profiles.md`](./reference/agent-profiles.md)

---

## Documentation index

**Audience:** *End-user* (run the CLI) · *Contributor* (change behavior) · *Maintainer* (CI, QA, release) · *Historical* (deferred specs / upstream context — not required reading).

| File | Topic | Audience | Coverage |
|------|-------|----------|----------|
| [`goclaw/README.md`](../goclaw/README.md) | Requirements, quick start, doc links | End-user | Shipped |
| [`README.md`](../README.md) (repo root) | Redirect to `goclaw/README.md` | End-user | Pointer |
| [`docs/goclaw/usage.md`](./goclaw/usage.md) | Modes, sessions, prompt/JSON, config, troubleshooting | End-user | Shipped |
| [`docs/goclaw/documentation.md`](./goclaw/documentation.md) | Where to add docs; house rules | Contributor | Shipped |
| [`docs/goclaw/philosophy.md`](./goclaw/philosophy.md) | UX principles, scope boundaries | Contributor | Shipped |
| [`docs/goclaw/changelog.md`](./goclaw/changelog.md) | User-visible version history | End-user | Shipped |
| [`docs/goclaw/roadmap.md`](./goclaw/roadmap.md) | Product checklist, CI notes | Maintainer | Shipped |
| [`docs/goclaw/security.md`](./goclaw/security.md) | Security notes (sync to onboarding embed) | Contributor | Shipped |
| [`docs/goclaw/coordinator.md`](./goclaw/coordinator.md) | D16 coordinator, `WorkerNotification` wire format | Contributor | Shipped |
| [`docs/goclaw/swarm.md`](./goclaw/swarm.md) | Disk mailbox hub vs coordinator | Contributor | Shipped |
| [`docs/goclaw/mcp-remote.md`](./goclaw/mcp-remote.md) | MCP bearer file, threats, future OAuth/WS | Contributor | Shipped + notes |
| [`docs/goclaw/ollama-stack.md`](./goclaw/ollama-stack.md) | Local Ollama stack, `compaction_model`, templates | End-user | Shipped |
| [`docs/goclaw/model-routing.md`](./goclaw/model-routing.md) | `task_models`, routers | End-user | Shipped |
| [`docs/goclaw/manual-tui-checklist.md`](./goclaw/manual-tui-checklist.md) | Bubble Tea / readline QA | Maintainer | Shipped |
| [`docs/goclaw/i18n.md`](./goclaw/i18n.md) | LLM language vs English UI | Contributor | Policy / planned |
| [`docs/goclaw/prefix-input-modes.md`](./goclaw/prefix-input-modes.md) | `!` / `@` / `&` / `/btw` prefix input (TUI + readline) | End-user | Shipped — see also [usage.md](./goclaw/usage.md#prefix-input----btw) |
| [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) | Rules, D1–D22, packages, env vars | Contributor | **Source of truth** |
| [`architecture.md`](./architecture.md) | English navigation hub | End-user | Shipped |
| [`agent-profiles.md`](./reference/agent-profiles.md) | Profiles, custom `.md` agents | Contributor | Shipped |
| [`hooks.md`](./reference/hooks.md) | Hook events, external + project file | Contributor | Shipped |
| [`mcp.md`](./reference/mcp.md) | MCP naming, transports, auth | Contributor | Partial (stdio + HTTP in goclaw; OAuth/WS future) |
| [`ide-bridge.md`](./reference/ide-bridge.md) | IDE integration design + goclaw §6–§7 | Contributor | Partial (notify + lockfile MCP) |
| [`retry-logic.md`](./reference/retry-logic.md) | HTTP retries, backoff | Contributor | Shipped |
| [`tool-contract.md`](./reference/tool-contract.md) | Tool caps, SSRF, loop budgets | Contributor | Shipped |
| [`tool-flows.md`](./reference/tool-flows.md) | Mermaid flows | Contributor | Shipped |
| [`code-adjustment-map.md`](./reference/code-adjustment-map.md) | Docs ↔ `internal/*` map | Maintainer | Shipped |
| [`custom-agents.md`](./reference/custom-agents.md) | Markdown agents + frontmatter | Contributor | D19 shipped; doc may exceed YAML in goclaw |
| [`coordinator-mode.md`](./reference/coordinator-mode.md) | Coordinator vs Team/Swarm | Contributor | D16 shipped; peer topology not shipped |
| [`yolo-classifier.md`](./reference/yolo-classifier.md) | Auto-mode gate | Contributor | Rule-based risk in goclaw; LLM classifier not separate |
| [`plugins.md`](./reference/plugins.md) | Plugin manifest, marketplace ideas | Contributor | Local plugin MVP; marketplace future |
| [`local-models.md`](./reference/local-models.md) | Ollama / hardware notes | Contributor | Reference |
| [`memory-system.md`](./reference/memory-system.md) | Memory subsystem detail | Contributor | FS memory shipped; extra extractors in doc |
| [`context-compaction.md`](./reference/context-compaction.md) | Compaction detail | Contributor | Heuristic + LLM compaction shipped |
| [`skills.md`](./reference/skills.md) | SKILL.md templates | Contributor | `internal/skills` injects SKILL.md |
| [`bash-security.md`](./reference/bash-security.md) | Shell layers vs MVP | Contributor | Reference |
| [`costs.md`](./reference/costs.md) | Cloud pricing notes | Contributor | Reference |
| [`practical-tips.md`](./reference/practical-tips.md) | UX/cost/product decisions (analyzed product) | Contributor | Reference |
| [`references.md`](./reference/references.md) | External link index (incl. OpenClaw GitHub for comparison) | Contributor | Reference |

---

## External references

Full link list: [`references.md`](./reference/references.md).

Conceptual source used during design: [claude-code-explain (helmcode)](https://claude-code-explain.helmcode.com/) — third-party analysis of Claude Code internals; goclaw implements a Go-native subset.

---

## Doc maintenance changelog

Edits to the documentation tree (index moves, link cleanups) are logged here so individual hubs do not duplicate history.

| Date | Change |
|------|--------|
| 2026-04-11 | Unified docs pass: normalized `docs/reference/` link labels to kebab-case filenames; replaced stale “ARCHITECTURE §” callouts with [CLAUDE.md](../goclaw/CLAUDE.md) / [roadmap.md](./goclaw/roadmap.md); language policy in [documentation.md](./goclaw/documentation.md); [README.md](../goclaw/README.md) defers detail to this map; shortened [architecture.md](./architecture.md) product blurb and “Entry point” row above; fixed [prefix-input-modes.md](./goclaw/prefix-input-modes.md) index row (shipped); [mcp-remote.md](./goclaw/mcp-remote.md) CLAUDE path corrected. |
| 2026-04-10 | [code-adjustment-map.md](./reference/code-adjustment-map.md) added; architecture hub delegates file index to this map. |
