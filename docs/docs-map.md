# Documentation Map

Single entry point for humans and AI agents: which file covers which topic, who it is for, and how it relates to shipped goclaw behavior.

**Source of truth for the goclaw binary:** [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) — architecture decisions D1–D22, coding conventions, roadmap. **Diagrams & module flow:** [`architecture.md`](./architecture.md).

**Project README (only):** [`goclaw/README.md`](../goclaw/README.md) — requirements, quick start, short links into this map. Repo root [`README.md`](../README.md) redirects there. **Where to add new docs:** [`docs/goclaw/documentation.md`](./goclaw/documentation.md). **This file (`docs-map.md`)** is the master file index (there is no `docs/README.md`). **Languages:** All docs are **English** — product docs in `docs/goclaw/`, design reference in `docs/reference/`.

---

## goclaw implementation status

| Area | Status |
|------|--------|
| Entry point | [`cmd/goclaw`](../goclaw/cmd/goclaw/main.go) → [`internal/cli`](../goclaw/internal/cli/root.go) (Cobra) → [`internal/app`](../goclaw/internal/app/run.go). **UI:** Bubble Tea TUI on TTY by default — flags and env in [`usage.md`](./goclaw/usage.md). Slash commands: [`internal/slashcmd`](../goclaw/internal/slashcmd/slash.go). |
| Packages | `internal/llm`, `orchestrator`, `session`, `tools`, `permissions`, `config`, `hooks`, `agents`, `memory`, `planfile`, `todos`, `mcp`, `ide`, `telegram` (optional Bot API bridge), `plugin`, `skills`, `ui/chat` (BubbleTea TUI) |
| Tools | Ten built-ins: `read_file`, `glob`, `grep`, `bash`, `write_file`, `edit_file`, `patch`, `web_fetch`, `web_search`, `todo_write`; optional `script` when `allow_script`; coordinator-only `spawn_agent`, `stop_task`; MCP tools as `mcp__<id>__<name>` |
| Plan workflow | Workspace `.goclaw/plan.md` ([`internal/planfile`](../goclaw/internal/planfile/planfile.go)); `/plan run` (save + execute) or `/apply-plan` switches to **`general-purpose`** for **one** execution turn (or **`coordinator`** with `--hub` / `plan_apply_use_coordinator`). Default **session** profile is **`general-purpose`** ([`config.Default()`](../goclaw/internal/config/config.go)) |
| Memory | `~/.goclaw/memory/` + `MEMORY.md` index; REPL `/memory list|add|delete`; opt-in auto-capture after `write_file`/`edit_file` (`memory_auto_extract`) |
| Compaction | Token-estimate heuristic (char/4), 0.85 threshold, 24-turn tail preserved; optional **`compaction_model`** + **`llm_compaction`** for LLM summaries ([`internal/orchestrator/compaction.go`](../goclaw/internal/orchestrator/compaction.go)) |
| Hooks | Same five events; Go `hooks.Registry`, **`external_hooks`** (subprocess stdin JSON or HTTP POST in settings), and project **`.goclaw/hooks.json`** when `trusted_workspace` is true ([`internal/hooks`](../goclaw/internal/hooks)) |
| MCP | stdio + streamable HTTP client — `mcp_servers` in merged settings ([`internal/mcp`](../goclaw/internal/mcp)); optional `bearer_token_file` per HTTP server; multi-server with per-server failure isolation |
| IDE | **Partial** — lockfile MCP + best-effort POST to `GOCLAW_IDE_NOTIFY_URL` (localhost-only); extension contract §7 [ide-bridge.md](./reference/ide-bridge.md) |
| Retries | `internal/llm/retry.go` — 10 attempts, 500 ms→5 min exp backoff, 429/503/504 (D22) |
| Profiles | 8 built-in in `internal/agents/profile.go` (includes `coordinator`, `code-review`) |
| V3 slice (partial vs full product docs) | Local plugins ([`internal/plugin`](../goclaw/internal/plugin)), SKILL.md runtime ([`internal/skills`](../goclaw/internal/skills)); **not** MCP OAuth/WS, remote marketplace, full IDE UI, or a shipped Team/Swarm implementation in this checkout |

---

## Reading order (implementation work)

1. [`docs/goclaw/usage.md`](./goclaw/usage.md) — run and configure the CLI
2. [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) — module state, decisions D1–D22, conventions
3. [`architecture.md`](./architecture.md) — package map, boot flow, orchestrator loop, coordinator workers, tool order
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
| [`docs/goclaw/swarm.md`](./goclaw/swarm.md) | Reference notes: swarm-style peer topology vs coordinator | Contributor | Reference only |
| [`docs/goclaw/mcp-remote.md`](./goclaw/mcp-remote.md) | MCP bearer file, threats, future OAuth/WS | Contributor | Shipped + notes |
| [`docs/goclaw/ollama-stack.md`](./goclaw/ollama-stack.md) | Local Ollama stack, `compaction_model`, templates | End-user | Shipped |
| [`docs/goclaw/model-routing.md`](./goclaw/model-routing.md) | `task_models`, routers | End-user | Shipped |
| [`docs/goclaw/code-review-workflow.md`](./goclaw/code-review-workflow.md) | `/review`, `code-review` profile, git-anchored review | End-user | Shipped |
| [`docs/goclaw/verification-recipe.md`](./goclaw/verification-recipe.md) | `.goclaw/verify.sh` post-edit checks | End-user | Shipped |
| [`docs/goclaw/ide-editor-setup.md`](./goclaw/ide-editor-setup.md) | Editor MCP lockfile golden path (`~/.goclaw/ide`, `ide_bridge_mcp`, notify URL) | End-user | Shipped |
| [`docs/goclaw/telegram-bridge.md`](./goclaw/telegram-bridge.md) | Optional `goclaw telegram bridge`: Bot API, allowlist, tool_permissions | End-user | Shipped |
| [`docs/goclaw/examples/ide-mcp-endpoint.example.json`](./goclaw/examples/ide-mcp-endpoint.example.json) | Copy-paste lockfile template for IDE HTTP MCP | End-user | Shipped |
| [`docs/goclaw/ide-pr-parity.md`](./goclaw/ide-pr-parity.md) | IDE / PR parity vs Wave A–B roadmap | Contributor | Shipped |
| [`docs/goclaw/manual-tui-checklist.md`](./goclaw/manual-tui-checklist.md) | Bubble Tea TUI QA | Maintainer | Shipped |
| [`docs/goclaw/i18n.md`](./goclaw/i18n.md) | LLM language vs English UI | Contributor | Policy / planned |
| [`docs/goclaw/prefix-input-modes.md`](./goclaw/prefix-input-modes.md) | `!` / `@` / `&` / `/btw` prefix input (TUI) | End-user | Shipped — see also [usage.md](./goclaw/usage.md#prefix-input----btw) |
| [`goclaw/CLAUDE.md`](../goclaw/CLAUDE.md) | Rules, D1–D22, packages, env vars | Contributor | **Source of truth** |
| [`architecture.md`](./architecture.md) | App/package diagrams, boot → `ChatRuntime` → orchestrator, coordinator vs workers | End-user · Contributor | Shipped |
| [`agent-profiles.md`](./reference/agent-profiles.md) | Profiles, custom `.md` agents | Contributor | Shipped |
| [`hooks.md`](./reference/hooks.md) | Hook events, external + project file | Contributor | Shipped |
| [`mcp.md`](./reference/mcp.md) | MCP naming, transports, auth | Contributor | Partial (stdio + HTTP in goclaw; OAuth/WS future) |
| [`ide-bridge.md`](./reference/ide-bridge.md) | IDE integration design + goclaw §6–§7 | Contributor | Partial (notify + lockfile MCP) |
| [`retry-logic.md`](./reference/retry-logic.md) | HTTP retries, backoff | Contributor | Shipped |
| [`tool-contract.md`](./reference/tool-contract.md) | Tool caps, SSRF, loop budgets | Contributor | Shipped |
| [`tool-flows.md`](./reference/tool-flows.md) | Mermaid flows | Contributor | Shipped |
| [`code-adjustment-map.md`](./reference/code-adjustment-map.md) | Docs ↔ `internal/*` map | Maintainer | Shipped |
| [`custom-agents.md`](./reference/custom-agents.md) | Markdown agents + frontmatter | Contributor | D19 shipped — see **Supported in goclaw** in doc (= `loader.go` keys); below that, upstream reference only |
| [`coordinator-mode.md`](./reference/coordinator-mode.md) | Coordinator vs Team/Swarm | Contributor | D16 shipped; peer topology not shipped |
| [`yolo-classifier.md`](./reference/yolo-classifier.md) | Auto-mode gate | Contributor | Rule-based risk in goclaw; LLM classifier not separate |
| [`plugins.md`](./reference/plugins.md) | Plugin manifest, marketplace ideas | Contributor | Local plugins **shipped**; remote marketplace **not implemented** |
| [`local-models.md`](./reference/local-models.md) | Ollama / hardware notes | Contributor | Reference |
| [`memory-system.md`](./reference/memory-system.md) | Memory subsystem detail | Contributor | FS memory shipped; extra extractors in doc |
| [`context-compaction.md`](./reference/context-compaction.md) | Compaction detail | Contributor | Heuristic + LLM compaction shipped |
| [`skills.md`](./reference/skills.md) | SKILL.md templates | Contributor | `internal/skills` injects SKILL.md |
| [`bash-security.md`](./reference/bash-security.md) | Shell layers vs current bash policy | Contributor | Reference |
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
| 2026-04-17 | Onboarding **agent profile** step; [usage.md](./goclaw/usage.md) Advanced + first-run + **mini plans** (`.goclaw/plans/`, `/plan new`, `/plan save|run [path]`); [manual-tui-checklist.md](./goclaw/manual-tui-checklist.md); [documentation.md](./goclaw/documentation.md) drift checklist; [memory-system.md](./reference/memory-system.md); [philosophy.md](./goclaw/philosophy.md); [code-adjustment-map.md](./reference/code-adjustment-map.md) §2; [CLAUDE.md](../goclaw/CLAUDE.md) slash summary; [changelog.md](./goclaw/changelog.md). |
| 2026-04-14 | [ide-editor-setup.md](./goclaw/ide-editor-setup.md) golden path + [examples/ide-mcp-endpoint.example.json](./goclaw/examples/ide-mcp-endpoint.example.json); [usage.md](./goclaw/usage.md) editor section; `goclaw doctor` **ide bridge:** block; [README.md](../goclaw/README.md) core link. |
| 2026-04-14 | **Doc + rules alignment (Ollama-only CLI, tools, IDE):** [architecture.md](./architecture.md), [roadmap.md](./goclaw/roadmap.md) quick reference, [philosophy.md](./goclaw/philosophy.md), [tool-contract.md](./reference/tool-contract.md), [custom-agents.md](./reference/custom-agents.md) supported YAML block, [costs.md](./reference/costs.md), [coordinator-mode.md](./reference/coordinator-mode.md), [code-adjustment-map.md](./reference/code-adjustment-map.md) §4 note, [ide-bridge.md](./reference/ide-bridge.md) §6.1 + §7; [documentation.md](./goclaw/documentation.md) security ↔ onboarding checklist; [docs-map.md](./docs-map.md) custom-agents index row; `.cursor/rules` (repo root) architecture + workflow. |
| 2026-04-14 | Added `code-review` profile, `/review`, and topic docs [code-review-workflow.md](./goclaw/code-review-workflow.md), [verification-recipe.md](./goclaw/verification-recipe.md), [ide-pr-parity.md](./goclaw/ide-pr-parity.md); [model-routing.md](./goclaw/model-routing.md) review-session section. |
| 2026-04-11 | Expanded [architecture.md](./architecture.md) (Mermaid flows); linked from [documentation.md](./goclaw/documentation.md) and this map (intro, reading order row 3, index row). |
| 2026-04-11 | **Release 1.3.0 (doc):** [roadmap.md](./goclaw/roadmap.md) shipped checklist Tiers 0–8 + optional follow-up waves; [changelog.md](./goclaw/changelog.md) **1.3.0**; [manual-tui-checklist.md](./goclaw/manual-tui-checklist.md) automated gate log; git tag `v1.3.0`. |
| 2026-04-11 | Unified docs pass: normalized `docs/reference/` link labels to kebab-case filenames; replaced stale “ARCHITECTURE §” callouts with [CLAUDE.md](../goclaw/CLAUDE.md) / [roadmap.md](./goclaw/roadmap.md); language policy in [documentation.md](./goclaw/documentation.md); [README.md](../goclaw/README.md) defers detail to this map; shortened [architecture.md](./architecture.md) product blurb and “Entry point” row above; fixed [prefix-input-modes.md](./goclaw/prefix-input-modes.md) index row (shipped); [mcp-remote.md](./goclaw/mcp-remote.md) CLAUDE path corrected. |
| 2026-04-10 | [code-adjustment-map.md](./reference/code-adjustment-map.md) added; architecture hub delegates file index to this map. |
