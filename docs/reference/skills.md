# Skills (reusable prompt flows) — reference and Go mapping

**Status in goclaw:** [`goclaw/internal/skills`](../../goclaw/internal/skills) discovers and injects `SKILL.md` content; see [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). Below is **reference-product** detail.

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (skills runtime). Third-party explainer: [Skills — claude-code-explain](https://claude-code-explain.helmcode.com/skills). Global map: [docs-map.md](../docs-map.md).

---

## 1. What a skill is (reference)

- A directory or file with **`SKILL.md`**: frontmatter (name, "use when…" description, allowed tools, optionally session hooks).
- The Markdown **body** instructs the flow; invocation via a **`/command`** in products with many slash commands.
- Integrates with [hooks.md](./hooks.md) when the frontmatter declares session hooks (`PostToolUse`, etc.).

---

## 2. Go mapping

| Piece | Location |
|-------|----------|
| `**/SKILL.md` discovery | `internal/skills` (v3 per [roadmap.md](../goclaw/roadmap.md)) |
| Injection into prompt | `internal/orchestrator` when activating a skill via `WithSkillsSnippet` |
| Slash commands | Optional: [plugins.md](./plugins.md) or own CLI; not part of the minimum core |

**Roadmap:** **v3** together with `internal/skills` and plugins; current contract: [CLAUDE.md](../../goclaw/CLAUDE.md) and the sections of this file.

---

## 3. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created stub: format, hooks, roadmap v3; docs map |
| 2026-04-12 | Translated from Spanish to English |
