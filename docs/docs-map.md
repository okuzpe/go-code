# Documentation Map

Single entry point for humans and AI agents: which file covers which topic, and whether it describes implemented goclaw behavior or a future design.

**Source of truth for the goclaw binary:** [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) — architecture decisions D0–D22, coding conventions, roadmap.

**Project README (only):** [`goclaw/README.md`](../goclaw/README.md) — requirements, quick start, doc links, topic table. Repo root [`README.md`](../README.md) redirects there. **Doc layout / naming:** [`docs/goclaw/documentation.md`](./goclaw/documentation.md). **This file (`docs-map.md`)** is the master file index (there is no `docs/README.md`).

---

## goclaw Implementation Status

| Area | Status |
|------|--------|
| Entry point | Thin `goclaw/cmd/goclaw` (`main.go` + `version.go`): slog + [`internal/cli`](../goclaw/internal/cli/root.go) Cobra tree + [`internal/app/run.go`](../goclaw/internal/app/run.go); **default interactive UI on a TTY** = Bubble Tea fullscreen TUI; opt out with **`GOCLAW_USE_TUI=0`** or **`--readline`** / **`GOCLAW_USE_READLINE=1`** for line-at-a-time REPL; explicit **`--tui`** / **`GOCLAW_USE_TUI=1`** forces TUI; flags `--profile`/`--session`/`--list-sessions`/`--no-tools`; slash commands in [`internal/slashcmd`](../goclaw/internal/slashcmd/slash.go) |
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

## Reading Order (for implementation work)

1. [`docs/goclaw/usage.md`](./goclaw/usage.md) — how to run and configure the CLI
2. [`docs/goclaw/documentation.md`](./goclaw/documentation.md) — doc layout and naming (optional)
3. [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) — module state, decisions D1–D22, conventions
4. [`architecture.md`](./architecture.md) — short English hub (navigation only)
5. [`code-adjustment-map.md`](./reference/code-adjustment-map.md) — which `docs/` files map to which `internal/*` packages when changing behavior
6. [`tool-contract.md`](./reference/tool-contract.md) — tool limits, network policy, loop budget
7. [`tool-flows.md`](./reference/tool-flows.md) — Mermaid diagrams: orchestrator loop, permissions, tool categories, coordinator, hooks
8. Deep dives: [`retry-logic.md`](./reference/retry-logic.md), [`hooks.md`](./reference/hooks.md), [`mcp.md`](./reference/mcp.md), [`agent-profiles.md`](./reference/agent-profiles.md)
9. Optional historical draft (Spanish, preserved **§** anchors): [`architecture-legacy-es.md`](./archive/architecture-legacy-es.md) — product scope §1, orchestrator §3.1, doc phases §4.4, decisions §5

---

## File Index

| File | Topic | goclaw |
|------|-------|--------|
| [`goclaw/README.md`](../goclaw/README.md) | **Only full README** — requirements, quick start, doc table, `docs/goclaw/` topic list | Implemented |
| [`README.md`](../README.md) (repo root) | Redirect to [`goclaw/README.md`](../goclaw/README.md) | Pointer |
| [`docs/goclaw/documentation.md`](./goclaw/documentation.md) | Doc taxonomy, goclaw vs monorepo `docs/` | Implemented |
| [`docs/goclaw/usage.md`](./goclaw/usage.md) | CLI workflows: modes, sessions, prompt/JSON, config, profiles, tools summary, hooks/MCP pointers | Implemented |
| [`docs/goclaw/ollama-stack.md`](./goclaw/ollama-stack.md) | Open-weight 7B/8B Ollama stack — project `.goclaw/` template, `compaction_model`, multi-model memory | Implemented |
| [`docs/goclaw/model-routing.md`](./goclaw/model-routing.md) | Per-turn `task_models` routing (`rules` / `llm`), precedence vs profile `model` | Implemented |
| [`docs/goclaw/roadmap.md`](./goclaw/roadmap.md) | Product checklist and CI notes | Implemented |
| [`docs/goclaw/philosophy.md`](./goclaw/philosophy.md) | UX principles and scope boundaries | Implemented |
| [`docs/goclaw/changelog.md`](./goclaw/changelog.md) | Version-to-version user-visible changes | Implemented |
| [`docs/goclaw/coordinator.md`](./goclaw/coordinator.md) | Coordinator mode (D16) — implementation map and `WorkerNotification` wire format | Implemented |
| [`docs/goclaw/mcp-remote.md`](./goclaw/mcp-remote.md) | MCP bearer file, threat notes, future OAuth/WS | Implemented notes + `bearer_token_file` |
| [`docs/goclaw/swarm.md`](./goclaw/swarm.md) | Swarm vs coordinator; disk mailbox hub | Implemented (`internal/swarm`) |
| [`docs/goclaw/manual-tui-checklist.md`](./goclaw/manual-tui-checklist.md) | Manual Bubble Tea / readline QA steps | Maintainers |
| [`docs/goclaw/i18n.md`](./goclaw/i18n.md) | LLM language matching vs English UI; future locale catalog | Planned / policy |
| [`docs/goclaw/prefix-input-modes.md`](./goclaw/prefix-input-modes.md) | Deferred epic: Claude-style `!`/`@`/`&` input; goclaw mapping | Deferred spec |
| [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) | Rules, D1–D22 condensed, package layout, conventions, roadmap | Source of truth |
| [`agent-profiles.md`](./reference/agent-profiles.md) | 7 built-in profiles (incl. `coordinator`) — tool filtering, system prompts, custom `.md` agents | Implemented |
| [`hooks.md`](./reference/hooks.md) | Hook event system — 5 events, Go handlers + `external_hooks` + `.goclaw/hooks.json` | Implemented |
| [`mcp.md`](./reference/mcp.md) | MCP naming, transports, auth (reference); **Implemented in goclaw** = stdio client + `mcp_servers` | Partial (stdio implemented; SSE/HTTP/OAuth future) |
| [`ide-bridge.md`](./reference/ide-bridge.md) | Full IDE integration design + goclaw §6–§7 contract | Partial in goclaw (notify + lockfile MCP + spec) |
| [`retry-logic.md`](./reference/retry-logic.md) | HTTP retry behavior — parameters, conditions, per-call budget | Implemented |
| [`tool-contract.md`](./reference/tool-contract.md) | Tool output limits, SSRF network policy, loop budgets | Implemented |
| [`tool-flows.md`](./reference/tool-flows.md) | Visual flows: orchestrator loop, permissions, tool categories, coordinator workers, hooks | Implemented |
| [`code-adjustment-map.md`](./reference/code-adjustment-map.md) | Maps `docs/` and `goclaw/internal/*` layers; post-change doc checklist | Maintainers |
| [`architecture.md`](./architecture.md) | English navigation hub; links to `goclaw/CLAUDE.md` and `docs/reference/` | Current |
| [`archive/README.md`](./archive/README.md) | Index of `docs/reference/` + archive layout from `docs/archive/` | Maintainers |
| [`architecture-legacy-es.md`](./archive/architecture-legacy-es.md) | Archived Spanish long-form spec (§1–§8), design exercises; links use `../` | Historical reference |

---

## Post-MVP and reference documents

For **goclaw’s shipped behavior**, use [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md). The table below marks **implemented in goclaw** vs **still broader / future** relative to this repo’s Go CLI.

| File | Topic | In goclaw today | Doc may still cover |
|------|--------|-----------------|---------------------|
| [`custom-agents.md`](./reference/custom-agents.md) | Markdown agents + frontmatter | **D19 implemented** (`~/.goclaw/agents`, `.goclaw/agents`) | Richer Claude Code–style frontmatter (MCP/hooks in YAML, priorities) |
| [`coordinator-mode.md`](./reference/coordinator-mode.md) | Coordinator vs Team/Swarm | **D16 hub-and-spoke implemented** — see [`docs/goclaw/coordinator.md`](./goclaw/coordinator.md) | Team/Swarm peer topology (not shipped) |
| [`yolo-classifier.md`](./reference/yolo-classifier.md) | Auto-mode safety gate | **D17-style rule-based risk** in `internal/permissions/risk.go` (`yolo_threshold`); not a separate LLM classifier | Lateral LLM classifier from reference product |
| [`mcp.md`](./reference/mcp.md) | MCP client reference | **stdio + streamable HTTP**, `mcp_servers`, loopback default | OAuth, WebSocket-only servers, enterprise policy |
| [`plugins.md`](./reference/plugins.md) | Plugin system | **Local plugin MVP** (`internal/plugin`) | Marketplace, full manifest merge as in reference doc |
| [`ide-bridge.md`](./reference/ide-bridge.md) | IDE integration | **Partial** — lockfile MCP, `GOCLAW_IDE_NOTIFY_URL` | Full editor parity, remote Bridge |
| [`local-models.md`](./reference/local-models.md) | Ollama / hardware | — | Reference |
| [`memory-system.md`](./reference/memory-system.md) | Memory subsystem detail | Filesystem memory + index implemented | Extra types / extractors from reference |
| [`context-compaction.md`](./reference/context-compaction.md) | Compaction detail | Heuristic compaction implemented | Reference numbers from analyzed product |
| [`skills.md`](./reference/skills.md) | SKILL.md templates | **`internal/skills` injects `SKILL.md`** | v3 roadmap in reference |

---

## External References

Full link list: [`references.md`](./reference/references.md).

Conceptual source used during design: [claude-code-explain (helmcode)](https://claude-code-explain.helmcode.com/) — a third-party analysis of Claude Code internals. The documents in this directory were informed by that analysis; goclaw implements a Go-native subset.

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: helmcode coverage table 00–21, MVP reading order, links. |
| 2026-04-07 | Cross-links: PRACTICAL_TIPS, OPENCLAW_AGENTS_AND_TOOLS; §2.6 ARCHITECTURE system-prompt anchor. |
| 2026-04-07 | Header: pointer to global inventory §8.0 in architecture draft (now [architecture-legacy-es.md](./archive/architecture-legacy-es.md)). |
| 2026-04-07 | MVP reading step 5 → TOOL_CONTRACT; Tools row links contract. |
| 2026-04-07 | OpenClaw section: no local clone; GitHub + claw-code/ note. |
| 2026-04-08 | Aligned to goclaw: source of truth → goclaw/CLAUDE.md; status summary; goclaw column; reading order updated; Memory/Compaction/Hooks/Retry/Tools/Slash/Agents corrected. |
| 2026-04-08 | Translated to English; replaced 21-row helmcode table with focused File Index; added Post-MVP Documents section; OpenClaw section removed. |
| 2026-04-08 | Status table: hooks external/MCP stdio/IDE partial; nine tools; `ui/chat`; File Index HOOKS/MCP/IDE; Post-MVP MCP row clarified. |
| 2026-04-09 | V3 slice: plugin/skills/swarm packages; MCP bearer; memory auto-capture; IDE_BRIDGE §7; docs `V3_MCP_REMOTE`, `SWARM`. |
| 2026-04-10 | goclaw topic docs under monorepo `docs/goclaw/`; `documentation.md` / `docs/README.md` / `docs/goclaw/README.md`; default UI = TUI on TTY; `architecture.md` hub + `docs/archive/` (`architecture-legacy-es.md` with `../` links); Post-MVP table aligned to shipped D16/D17/D19; `references.md` index. |
| 2026-04-10 | Operator docs (`usage`, `documentation`, `roadmap`, `philosophy`, `changelog`) moved from `goclaw/*.md` to `docs/goclaw/` (kebab-case); `goclaw/` keeps `README.md` + `CLAUDE.md` only. |
| 2026-04-10 | Documentation UX: `docs/README.md` role-based entry; Diátaxis-style principles in `docs/goclaw/documentation.md`; `goclaw/README.md` order = requirements → quick start → full doc index. |
| 2026-04-10 | Single canonical README: **`goclaw/README.md`**; repo root is a short redirect; **`docs/README.md`** and **`docs/goclaw/README.md`** removed; topic file table in `goclaw/README.md`. |
| 2026-04-10 | Link audit: fixed `](goclaw/...)` → correct `../` / `../../` prefixes from `docs/`, `docs/reference/`, `docs/openclaw/`; fixed **`docs/archive/README.md`** and **`architecture-legacy-es.md`** (module vs `docs/goclaw/`). |
| 2026-04-10 | Added [`reference/code-adjustment-map.md`](./reference/code-adjustment-map.md) — docs-to-code layer routes; reading order step 5; File Index row. |
| 2026-04-10 | Added [`reference/tool-flows.md`](./reference/tool-flows.md); reading order after `tool-contract`; status table tools row (10 + optional `script`); File Index row. |
