# goclaw — Philosophy

goclaw is a local-first CLI coding agent. The goal is daily-driver usability: predictable behavior, clear feedback, and safe tool execution.

## Principles

- **Human-readable by default**: tool activity should be understandable at a glance (no raw JSON in the UI).
- **State, not stories**: show what the agent is doing (“searching”, “reading”, “running”) without verbose narration.
- **Tools are bounded**: every tool has caps and safety checks; failures should be explicit and recoverable.
- **Terminal is transport**: UI details can vary (readline vs TUI), but the underlying behavior should be consistent.
- **Workdir matters**: file operations are workspace-scoped; “where you are” is part of correctness.
- **Small, composable surfaces**: keep interfaces minimal; prefer predictable contracts over clever behavior.

## What this project is not

- Not a **Team/Swarm** product (tmux-style peer agents or external job grids). In-process **coordinator** delegation (`spawn_agent`) is intentional and bounded; see [coordinator.md](./coordinator.md) and [coordinator-mode.md](../reference/coordinator-mode.md). The minimal disk mailbox hub (`internal/swarm`) is separate; see [swarm.md](./swarm.md).
- Not a plugin marketplace.
- Not a “cloud-first” agent that requires external services to be usable (Anthropic is optional).

## Documentation (monorepo)

Cross-cutting specs: **`docs/reference/`**. Topic notes: **`docs/goclaw/`** (this file). **Canonical entry:** **[README.md](../../goclaw/README.md)**; master index: **[`docs-map.md`](../docs-map.md)**. Operator guide: **[usage.md](usage.md)**; map: **[documentation.md](documentation.md)**; checklist: **[roadmap.md](roadmap.md)**; history: **[changelog.md](changelog.md)**; implementation truth: **[CLAUDE.md](../../goclaw/CLAUDE.md)**.
