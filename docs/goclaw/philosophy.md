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
- Not a **multi-channel gateway** (Discord, Telegram, mobile pairing, long-lived daemon as the primary UX). goclaw is REPL/TUI-first; a control-plane gateway would be a different product shape. See [roadmap.md — Future transport and scale](roadmap.md#future-transport-and-scale).

## Lessons from wider agent stacks

These patterns were distilled from comparing goclaw to larger **Node/TS** agent repos (multi-channel products such as [openclaw/openclaw](https://github.com/openclaw/openclaw)). They inform **how we extend goclaw**, not code to port.

- **Web tools:** Treat SSRF, redirect chains, response size, timeouts, and search-provider fallbacks as **first-class test targets**—same discipline as dedicated fetch/search test suites upstream. Implemented direction: [tool-contract.md](../reference/tool-contract.md), `internal/tools` tests.
- **MCP at the edge:** Prefer **stdio / HTTP MCP** and local plugins for new capability instead of growing the core loop. See [mcp.md](../reference/mcp.md), [mcp-remote.md](./mcp-remote.md), D6 in [CLAUDE.md](../../goclaw/CLAUDE.md).
- **Declarative surface area:** Custom agents and SKILL.md keep prompts and bundles **out of the hot path** of the orchestrator. See [custom-agents.md](../reference/custom-agents.md), [skills.md](../reference/skills.md).
- **Routing stays thin:** Session + tool policy should stay a **small mapping** (which tools, which memory, which profile)—even when there is only one interactive channel today.
- **If multi-transport ever matters:** Prefer **one adapter per channel** (queue + allowlist) rather than mixing socket/UI logic into `internal/orchestrator`.

## Documentation (monorepo)

Cross-cutting specs: **`docs/reference/`**. Topic notes: **`docs/goclaw/`** (this file). **Canonical entry:** **[README.md](../../goclaw/README.md)**; master index: **[`docs-map.md`](../docs-map.md)**. Operator guide: **[usage.md](usage.md)**; map: **[documentation.md](documentation.md)**; checklist: **[roadmap.md](roadmap.md)**; history: **[changelog.md](changelog.md)**; implementation truth: **[CLAUDE.md](../../goclaw/CLAUDE.md)**.
