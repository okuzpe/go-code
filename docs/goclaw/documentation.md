# Where to put documentation (contributors)

The **master file index** for the whole `docs/` tree is [`docs-map.md`](../docs-map.md) — start there for “what exists” and reading order.

**Implementation source of truth** for the Go module: [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) (packages, D1–D22, env vars, conventions).

**Architecture diagrams & app flow (repo hub):** [`architecture.md`](../architecture.md) — package map, boot path, `ChatRuntime`, orchestrator loop, coordinator vs workers, tool registration order.

**Language:** All documentation in this repository is **English** — same rule as CLAUDE.md code comments. `docs/reference/` files are design-level reference notes; keep them in English.

**Terminology:** Prefer **shipped** / **not implemented** / **Partial** (see [`docs-map.md`](../docs-map.md)) over legacy **MVP** or **post-MVP** labels when describing goclaw — those phases referred to an older planning vocabulary; the code is the source of truth.

## Layout (one line each)

| Layer | Path | Put here |
|-------|------|----------|
| Landing | [`goclaw/README.md`](../../goclaw/README.md) | Pitch, requirements, quick start, links |
| Explanation (flows) | [`architecture.md`](../architecture.md) | Mermaid: packages, boot, orchestrator, coordinator; link from here and [docs-map.md](../docs-map.md), do not duplicate long diagrams in `usage.md` |
| Operators | [`usage.md`](usage.md) | Run modes, flags, sessions, config, troubleshooting |
| Topic notes | `docs/goclaw/*.md` (kebab-case) | goclaw-specific deep dives (coordinator wire format, MCP notes, [ide-editor-setup.md](./ide-editor-setup.md), …) |
| Cross-cutting contracts | `docs/reference/*.md` | Tool contracts, hooks, MCP reference, diagrams — shared vocabulary |

## House rules

- **Diátaxis:** split tutorial (quick start), how-to ([`usage.md`](usage.md)), reference ([`CLAUDE.md`](../../goclaw/CLAUDE.md), `docs/reference/`), and explanation ([`philosophy.md`](philosophy.md), [`architecture.md`](../architecture.md)) instead of duplicating the same table in multiple places.
- **Do not** paste full tool tables or the full D1–D22 matrix into `usage.md` — summarize and **link** to `CLAUDE.md`.
- **Security copy sync (checklist):** when you change [`security.md`](security.md), **always** copy the updated body into [`goclaw/internal/app/onboarding_security_full.md`](../../goclaw/internal/app/onboarding_security_full.md) before merging (see HTML comment at top of that embed file). Same rule is referenced from [code-adjustment-map.md](../reference/code-adjustment-map.md) §17.
- **Naming:** prefer **kebab-case** Markdown filenames under `docs/`. When you add or rename a top-level doc, update **`docs-map.md`**.

## Changelog

Merged into **[Doc maintenance changelog](../docs-map.md#doc-maintenance-changelog)**.
