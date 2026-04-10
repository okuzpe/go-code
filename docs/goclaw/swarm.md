# Swarm (V3+) vs coordinator

## Coordinator (existing)

Hub-and-spoke: the **`coordinator`** profile uses **`spawn_agent`** and **`stop_task`** only. Workers are separate processes/sessions orchestrated from one hub. See [coordinator.md](coordinator.md) and [coordinator-mode.md](../reference/coordinator-mode.md).

## Swarm (minimal implementation)

**Package:** [`internal/swarm`](../../goclaw/internal/swarm/) — a **disk-backed mailbox hub** for peer-style messaging **without** using `spawn_agent`.

- A **hub** directory contains one subdirectory per recipient id (e.g. agent or lane name).
- **`Post(from, to, body)`** writes a JSON message file into the recipient inbox.
- **`ReadSince(to, afterID)`** returns messages with id greater than `afterID`, ordered by id.

This is intentionally small: tests and future tools can build richer policy (permissions, caps) on top. It does **not** replace the coordinator tool surface.

## When to use which

| Need | Use |
|------|-----|
| Delegate a task to a worker agent with tools | Coordinator `spawn_agent` |
| Persisted mailbox between named peers / experiments | `internal/swarm` hub |
