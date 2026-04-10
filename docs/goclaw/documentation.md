# goclaw documentation map

This file explains **what documentation exists**, **where it belongs**, and **what not to duplicate**. The **implementation source of truth** for agents and contributors is still **[CLAUDE.md](../../goclaw/CLAUDE.md)**.

**Layout:** Operator and product Markdown for goclaw lives in **`docs/goclaw/`** (this folder). **`goclaw/`** next to `go.mod` keeps **[README.md](../../goclaw/README.md)** (landing) and **[CLAUDE.md](../../goclaw/CLAUDE.md)** (implementation source of truth) only.

## Principles (how we write docs)

Documentation follows [Diátaxis](https://diataxis.fr/): keep **tutorials** (first success), **how-to guides** (task-oriented), **reference** (exhaustive facts), and **explanation** (why and trade-offs) in different places so readers find the right depth.

| Kind | In this repo | Examples |
|------|----------------|----------|
| Tutorial | Short copy-paste on [README.md](../../goclaw/README.md) and in [usage.md](usage.md) (quick start) | `go run ./cmd/goclaw doctor` |
| How-to | [usage.md](usage.md), topic files in this folder | Sessions, Anthropic, hooks, MCP |
| Reference | [CLAUDE.md](../../goclaw/CLAUDE.md), [docs/reference/](../reference/) | Tool contract, env vars, D1–D22 |
| Explanation | [philosophy.md](philosophy.md), [architecture.md](../architecture.md), [coordinator-mode.md](../reference/coordinator-mode.md) | Scope, architecture, coordinator vs swarm |

**House rules:** prefer **one main topic per file**; **kebab-case** names under `docs/`; **link** instead of duplicating long tables (e.g. usage summarizes tools, CLAUDE is canonical); when you add or rename a top-level doc, update **[docs-map.md](../docs-map.md)**.

---

## Layers (where to put new docs)

| Layer | Path | Purpose | Audience |
|-------|------|---------|----------|
| Landing | [README.md](../../goclaw/README.md) | Short pitch, quick start, pointer to the rest | Everyone |
| Operators | [usage.md](usage.md) | Run modes, flags, sessions, config, troubleshooting | Humans using the CLI |
| Implementors / AI agents | [CLAUDE.md](../../goclaw/CLAUDE.md) | Packages, D1–D22, tool contract, conventions, roadmap | Contributors, coding agents |
| Product checklist | [roadmap.md](roadmap.md) | Done / pending features, CI notes | Maintainers |
| Principles | [philosophy.md](philosophy.md) | UX and scope boundaries | Maintainers |
| Releases | [changelog.md](changelog.md) | Version-to-version user-visible changes | Users, packagers |
| Topic deep dives | **Topic files** table in [README.md](../../goclaw/README.md) | Coordinator, MCP notes, swarm, manual QA, JSON templates | Contributors |
| `docs/` tree | No `README` — use [README.md](../../goclaw/README.md) + [docs-map.md](../docs-map.md) | Markdown under `reference/`, `goclaw/`, `archive/`, … | Everyone |
| Master index | [docs-map.md](../docs-map.md) | Reading order, file index, implementation status | Everyone |
| Agent skill prompts | [.claude/skills/](../../goclaw/.claude/skills/) | Reusable task prompts for Claude Code / Cursor (not a user manual) | AI workflows only |
| Script runbooks | [scripts/*.md](../../goclaw/scripts/MOCK_PARITY_HARNESS.md) | How to run harnesses next to scripts | CI / maintainers |

**Monorepo** (parent of `goclaw/`): cross-cutting specs live under **[docs/reference/](../reference/)** — e.g. [architecture.md](../architecture.md), [archive/architecture-legacy-es.md](../archive/architecture-legacy-es.md), [tool-contract.md](../reference/tool-contract.md), [mcp.md](../reference/mcp.md), [hooks.md](../reference/hooks.md), [agent-profiles.md](../reference/agent-profiles.md), [coordinator-mode.md](../reference/coordinator-mode.md). Product-level concepts in `docs/reference/`; goclaw’s **Go mapping** lives in **CLAUDE.md** and **docs/goclaw/**.

---

## Markdown inventory (goclaw + linked docs)

Scope: **`goclaw/`** and monorepo **`docs/`** Markdown for GoClaw (excludes `claw-code/`).

| Category | Paths |
|----------|--------|
| Landing & operators | `goclaw/README.md`, [usage.md](usage.md), [philosophy.md](philosophy.md), [changelog.md](changelog.md) |
| Implementors / AI agents | `goclaw/CLAUDE.md`, [roadmap.md](roadmap.md), [documentation.md](documentation.md) |
| Topic deep dives | `docs/goclaw/*.md` — **kebab-case**; listed in [README.md](../../goclaw/README.md) |
| Embedded in product | `internal/app/onboarding_*.md`, `internal/orchestrator/base_system_prompt.md` |
| Security copy (sync) | [`goclaw/internal/app/onboarding_security_full.md`](../../goclaw/internal/app/onboarding_security_full.md) must match [security.md](security.md) — edit **security.md** first, then mirror into the embed (see HTML comment at top of the embed file). |
| Workflow skills | `.claude/skills/*.md`, `.cursor/skills/**/*.md` (not end-user manuals) |
| Scripts | `scripts/MOCK_PARITY_HARNESS.md` |
| Monorepo docs | See [docs-map.md](../docs-map.md) File Index — `docs/reference/`, `docs/openclaw/`, `docs/archive/` |
| Archive folder | [docs/archive/README.md](../archive/README.md); [architecture-legacy-es.md](../archive/architecture-legacy-es.md) |

---

## What should be in goclaw vs monorepo docs

| Put in **goclaw/** | Put under **docs/** |
|--------------------|---------------------|
| How this **Go module** is wired (packages, flags, env vars) | Generic tool contracts and patterns that apply beyond this module |
| Topic notes tied to **this codebase** (coordinator wire format, MCP bearer notes) | Long-form reference (e.g. full IDE bridge vision), OpenClaw notes, Spanish archive |
| `.claude/skills` for **this repo’s** workflows | N/A |

**Avoid duplicating** the full tool table or D1–D22 tables in usage.md — it should **summarize** and **link** to CLAUDE.md.

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-10 | Added DOCUMENTATION.md and topic layout; monorepo pointers to `docs/` hub + `docs/archive/`. |
| 2026-04-10 | Topic docs consolidated under monorepo **`docs/goclaw/`** (no `goclaw/docs/`). |
| 2026-04-10 | Added **Principles** (Diátaxis-style) and house rules for where tutorials / how-to / reference / explanation live. |
| 2026-04-10 | **README cleanup:** removed `docs/README.md` and `docs/goclaw/README.md`; only [README.md](../../goclaw/README.md) (full) + repo-root [README.md](../../README.md) (pointer). Topic table stays in `goclaw/README.md`. |
| 2026-04-10 | Documented **onboarding_security_full.md** ↔ **security.md** sync; first-run onboarding covered in [usage.md](usage.md). |
