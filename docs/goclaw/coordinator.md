# Coordinator mode (D16) — implementation reference

**Advanced product note:** The normal local-first path is **`build`** for direct coding and **`plan`** for read-only planning first. **`coordinator`** is an advanced hub-and-spoke profile for explicit delegation via `spawn_agent`. Internally, `build` still resolves to `general-purpose` for compatibility. **`make run-hub`** in this repo selects coordinator for convenience.

This document ties [coordinator-mode.md](../reference/coordinator-mode.md) (design reference) to the **implemented** hub-and-spoke coordinator in goclaw. It complements the plan → execute workflow ([`internal/planfile`](../internal/planfile/), `/apply-plan`) and stays separate from future **Team/Swarm** designs.

## Implementation map

| Concept | Location |
|---------|----------|
| `spawn_agent` tool (isolated worker session, no nesting) | [`internal/coordinator/spawn_agent.go`](../internal/coordinator/spawn_agent.go) |
| `stop_task` tool | [`internal/coordinator/stop_task.go`](../internal/coordinator/stop_task.go) |
| Worker cancel registry | [`internal/coordinator/worker_registry.go`](../internal/coordinator/worker_registry.go) |
| Interactive workers (`interactive: true`), inbox + `/focus` | [`internal/coordinator/interactive.go`](../internal/coordinator/interactive.go), [`internal/coordinator/focus.go`](../internal/coordinator/focus.go) |
| Coordinator profile (`spawn_agent`, `stop_task`, `todo_write` only) | [`internal/agents/profile.go`](../internal/agents/profile.go) (`Coordinator`) |
| Wiring into chat runtime | [`internal/app/chat_wiring.go`](../internal/app/chat_wiring.go) |

## Design constraints (from product docs)

- **Coordinator** does not use `read_file`, `write_file`, `edit_file`, or `bash` directly — only delegation tools ([coordinator-mode.md](../reference/coordinator-mode.md) §2.1–2.2).
- **Workers** use a normal (or narrowed) profile and **do not** see the full user ↔ coordinator transcript; each `task` string must be self-contained (§2.7).
- Do **not** merge Coordinator routing with Team/Swarm mailboxes in the same abstraction (§1).

## Wire type: worker result (as implemented)

The `spawn_agent` tool returns JSON matching `WorkerNotification`:

```go
type WorkerNotification struct {
	TaskID  string `json:"task_id"` // worker session id; use with stop_task
	Profile string `json:"profile"`
	Status  string `json:"status"`  // "completed" | "failed" | "running" (interactive)
	Summary string `json:"summary"` // first non-empty line of the result
	Result  string `json:"result"`  // full worker response text
}
```

The coordinator LLM receives this as a `tool_result` and synthesizes it for the user. Token `usage` is not attached in the current struct (optional future extension).

When **`interactive`** is `true` on `spawn_agent`, the tool returns immediately with `status: "running"`; the worker continues in a background goroutine. Users send further input via **`/focus <task_id_prefix>`** in the REPL (and **`/detach`** to return to the parent). Each message runs an additional `RunStreaming` turn on the worker session.

## Coordinator profile (implemented)

| Field | Value |
|-------|-------|
| `Name` | `coordinator` |
| `ToolAllowlist` | `spawn_agent`, `stop_task`, `todo_write` |
| `ReadOnly` | `true` (no direct file/shell tools) |
| `SystemPrompt` | Delegation-first; workers hold the full toolbox |

Workers are started with a registry **without** `spawn_agent` to prevent coordinator nesting.

### Worker context (workspace parity)

Each worker orchestrator receives the same **workspace root**, **project context** (manifest + README + `CLAUDE.md` excerpt via `buildProjectContext`), **shared memory store** (`~/.goclaw/memory/`), and **skills snippet** as the parent chat session (`PrepareChatRuntime` wires `WithWorkdir`, `WithProjectContext`, `WithMemoryStore`, `WithSkillsSnippet` on `SpawnAgentTool`). Workers still **do not** inherit the coordinator’s MCP-registered tools or the hub transcript — only built-ins on the worker registry — by design (nesting safety and predictable tool surface).

## Phases (reference mapping)

| Phase | Owner | goclaw |
|-------|--------|--------|
| Research | Workers (parallel) | Multiple `spawn_agent` calls / parallel tool execution where the model issues parallel tool calls |
| Synthesis | Coordinator | REPL thread; often `plan` profile + `.goclaw/plan.md` |
| Implementation | Workers | `/apply-plan` handoff to `build` (internal alias: `general-purpose`) or other worker profiles |
| Verification | Workers | `verification` profile in separate worker runs |

## Tests and harness

- Unit / integration tests: [`internal/coordinator/spawn_agent_test.go`](../internal/coordinator/spawn_agent_test.go), [`internal/coordinator/stop_task_test.go`](../internal/coordinator/stop_task_test.go).
- Mock OpenAI-compat LLM: [`testutil/mockopenai`](../testutil/mockopenai/).
- Scripted checklist: [`scripts/run_mock_parity_harness.sh`](../scripts/run_mock_parity_harness.sh) and [`scripts/mock_parity_scenarios.json`](../scripts/mock_parity_scenarios.json).

## Changelog

| Date | Change |
|------|--------|
| 2026-04-08 | Initial sketch (pre-code). |
| 2026-04-09 | Updated to reflect implemented `internal/coordinator`, links to code and parity harness. |
| 2026-04-10 | Renamed file to `coordinator.md` from the earlier working name. |
| 2026-04-10 | Documented interactive `spawn_agent`, `running` status, `/focus` / `/detach`, and default hub profile. |
| 2026-04-11 | Documented worker project context, memory, skills injection; MCP not inherited on workers. |
