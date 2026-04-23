# Swarm notes vs coordinator

## Coordinator (existing)

Hub-and-spoke: the **`coordinator`** profile uses **`spawn_agent`** and **`stop_task`** only. Workers are separate processes/sessions orchestrated from one hub. See [coordinator.md](coordinator.md) and [coordinator-mode.md](../reference/coordinator-mode.md).

## Swarm (reference-only in this checkout)

There is **no** `internal/swarm` package in the current repository checkout. This document is kept as a **reference note** for a peer-style topology that is distinct from the shipped coordinator workflow.

If a future implementation is added, the intent would still be a **disk-backed mailbox hub** for peer-style messaging **without** using `spawn_agent`.

Until then, treat any swarm terminology in `docs/reference/` as design context, not shipped CLI behavior.

## When to use which

| Need | Use |
|------|-----|
| Delegate a task to a worker agent with tools | Coordinator `spawn_agent` |
| Compare against a peer-topology design | This reference doc only |
