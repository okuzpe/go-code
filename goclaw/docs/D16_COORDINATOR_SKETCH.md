# D16 Coordinator mode — implementation sketch (pre-code)

This document narrows [COORDINATOR_MODE.md](../../COORDINATOR_MODE.md) into concrete Go shapes for a future **hub-and-spoke** coordinator. It is **not** implemented in goclaw today; it exists so the plan → execute workflow ([`internal/planfile`](../internal/planfile/), `/apply-plan`) can evolve without mixing Coordinator semantics with Team/Swarm.

## Design constraints (from product docs)

- **Coordinator** must not use `read_file`, `write_file`, `edit_file`, or `bash` directly — only delegation tools and messaging ([COORDINATOR_MODE.md](../../COORDINATOR_MODE.md) §2.1–2.2).
- **Workers** get the normal toolbox (or a narrowed worker profile) and **must not** see the full user ↔ coordinator transcript; each task message must be self-contained (§2.7).
- Do **not** merge Coordinator routing with Team/Swarm mailboxes in the same abstraction (§1).

## Proposed wire type: worker result

JSON (or XML-like envelope) the worker runner sends back to the coordinator loop:

```json
{
  "task_id": "uuid-or-slug",
  "status": "completed",
  "summary": "one-line outcome for the coordinator",
  "result": "detailed text or structured payload",
  "usage": { "prompt_tokens": 0, "completion_tokens": 0 }
}
```

`status` values: `completed`, `failed`, `killed`.

**Go shape (illustrative):**

```go
type WorkerNotification struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Result  string `json:"result"`
	Usage   any    `json:"usage,omitempty"`
}
```

The orchestrator (or a thin `internal/coordinator` package) would parse this, update an in-memory registry of tasks, and **summarize** into the coordinator session instead of dumping full worker transcripts by default.

## Coordinator profile (illustrative)

| Field | Proposed value |
|-------|----------------|
| `ToolAllowlist` | Delegation-only tools (names TBD), e.g. spawn/continue worker, stop task — **not** file/shell tools |
| `ReadOnly` | `true` at the filesystem layer (no direct mutation tools) |
| `SystemPrompt` | Coordinator-only: synthesize specs, delegate with verbatim paths and line numbers |

Worker profile: reuse `general-purpose` or a dedicated `worker` profile with full tools minus coordinator-only tools.

## Phases (reference mapping)

| Phase | Owner | goclaw direction |
|-------|--------|------------------|
| Research | Workers (parallel) | Multiple `Run` contexts with isolated `session.Session` |
| Synthesis | Coordinator | Current REPL thread; may use `plan` profile + `.goclaw/plan.md` |
| Implementation | Workers | `/apply-plan` today approximates a **single-threaded** handoff to `general-purpose` |
| Verification | Workers | `verification` profile in separate turns or worker runs |

## Dependencies before coding D16

- Stable **task id** generation and cancellation (context + optional process kill for subprocess workers).
- Tests with [`testutil/mockserver`](../testutil/mockserver/) for multi-turn coordinator + fake worker responses.
- Permission policy: coordinator tools subject to the same `ask` / `allow` / `deny` rules as built-ins.

## Changelog

| Date | Change |
|------|--------|
| 2026-04-08 | Initial sketch: WorkerNotification JSON, profile split, phase mapping, dependencies. |
