# Where to put documentation (contributors)

The **master file index** for the whole `docs/` tree is [`docs-map.md`](../docs-map.md) — start there for “what exists” and reading order.

**Implementation source of truth** for the Go module: [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) (packages, D1–D22, env vars, conventions).

**Architecture diagrams & app flow (repo hub):** [`architecture.md`](../architecture.md) — package map, boot path, `ChatRuntime`, orchestrator loop, coordinator vs workers, tool registration order.

**Language:** Product and operator docs under **`docs/goclaw/`** are **English** (match the CLI UI strings). Cross-cutting design notes under **`docs/reference/`** are mostly **Spanish** today (comparison with Claude Code–style stacks); treat them as reference, not UI copy. A future pass may translate or split `reference-es/` — until then, keep new **product** prose in English.

**Terminology:** Prefer **shipped** / **not implemented** / **Partial** (see [`docs-map.md`](../docs-map.md)) over legacy **MVP** or **post-MVP** labels when describing goclaw — those phases referred to an older planning vocabulary; the code is the source of truth.

## Layout (one line each)

| Layer | Path | Put here |
|-------|------|----------|
| Landing | [`goclaw/README.md`](../../goclaw/README.md) | Pitch, requirements, quick start, links |
| Explanation (flows) | [`architecture.md`](../architecture.md) | Mermaid: packages, boot, orchestrator, coordinator; link from here and [docs-map.md](../docs-map.md), do not duplicate long diagrams in `usage.md` |
| Operators | [`usage.md`](usage.md) | Run modes, flags, sessions, config, troubleshooting |
| Topic notes | `docs/goclaw/*.md` (kebab-case) | goclaw-specific deep dives (coordinator wire format, MCP notes, …) |
| Cross-cutting contracts | `docs/reference/*.md` | Tool contracts, hooks, MCP reference, diagrams — shared vocabulary |
| Deferred / historical topic notes | `docs/goclaw/*.md` (e.g. [prefix-input-modes.md](prefix-input-modes.md)) | Specs not shipped; keep out of the default reading path |

## House rules

- **Diátaxis:** split tutorial (quick start), how-to ([`usage.md`](usage.md)), reference ([`CLAUDE.md`](../../goclaw/CLAUDE.md), `docs/reference/`), and explanation ([`philosophy.md`](philosophy.md), [`architecture.md`](../architecture.md)) instead of duplicating the same table in multiple places.
- **Do not** paste full tool tables or the full D1–D22 matrix into `usage.md` — summarize and **link** to `CLAUDE.md`.
- **Security copy sync:** edit [`security.md`](security.md) first, then mirror into [`goclaw/internal/app/onboarding_security_full.md`](../../goclaw/internal/app/onboarding_security_full.md) (see HTML comment at top of that file).
- **Naming:** prefer **kebab-case** Markdown filenames under `docs/`. When you add or rename a top-level doc, update **`docs-map.md`**.

## Changelog

Merged into **[Doc maintenance changelog](../docs-map.md#doc-maintenance-changelog)**.
