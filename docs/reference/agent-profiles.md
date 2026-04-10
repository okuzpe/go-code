# Agent Profiles

A **profile** controls how the orchestrator behaves for a given task. Each profile configures:

- **Model** — inherits the global config by default; `ModelOverride` pins a specific model.
- **Tool allowlist** — the subset of registered tools offered to the LLM (`nil` = all tools; empty slice = no tools).
- **Read-only flag** — when `true`, `bash` is removed from the tool list even if the allowlist includes it.
- **System prompt** — prepended to every request to shape the model's behavior.

Select a profile with `-profile <name>` or set `agent_profile` in `settings.json`. Implementation: [`goclaw/internal/agents/profile.go`](../../goclaw/internal/agents/profile.go).

**Shared prefix (all profiles):** [`internal/orchestrator/orchestrator.go`](../../goclaw/internal/orchestrator/orchestrator.go) `baseSystemPrompt` instructs the model to prefer dedicated tools (**D12** / [ARCHITECTURE_LEGACY_ES.md §2.1](../archive/architecture-legacy-es.md)) — `read_file`, `glob`, `grep`, `write_file`, `edit_file`, `web_fetch`, `web_search`, `todo_write` — and use `bash` only when no dedicated tool fits.

### Plan → execute workflow (v1)

1. Use **`plan`** (or author manually) to produce a numbered Markdown plan; save it at **`.goclaw/plan.md`** in the workspace (template: `/plan init` or `/plan template` in the REPL).
2. Run **`/apply-plan`** (optional path argument) to switch to **`general-purpose`** and send one user turn that embeds the plan and instructs the model to execute step by step.
3. Alternatively, use **`/profile general-purpose`** after planning and paste the plan, or resume execution in a new session with the saved file.

**Coordinator (D16)** is **implemented** in goclaw: profile `coordinator`, tools `spawn_agent` / `stop_task`, isolated worker sessions — see [docs/goclaw/coordinator.md](../goclaw/coordinator.md) and [coordinator-mode.md](./coordinator-mode.md) (concept vs product reference).

---

## Built-in Profiles

Seven built-ins in [`goclaw/internal/agents/profile.go`](../../goclaw/internal/agents/profile.go). Custom Markdown agents can override names; see **Custom agents** below. **Default** `agent_profile` when unset in settings is **`coordinator`** ([`config.Default()`](../../goclaw/internal/config/config.go)); use **`general-purpose`** when you want a single agent with full tools.

| Profile | `-profile` value | Tool allowlist | Read-only | Default? |
|---------|-----------------|----------------|-----------|----------|
| General-Purpose | `general-purpose` | All tools | No | — |
| Explore | `explore` | read_file, glob, grep, web_fetch, web_search, todo_write | Yes | — |
| Plan | `plan` | read_file, glob, grep, web_search, todo_write | Yes | — |
| Verification | `verification` | read_file, bash, todo_write | No | — |
| Guide | `guide` | (none) | Yes | — |
| StatusLine | `statusline` | (none) | Yes | — |
| Coordinator | `coordinator` | spawn_agent, stop_task, todo_write | Yes | Yes |

---

## Profile Details

### general-purpose

```
Go var:        agents.GeneralPurpose
Tool allowlist: nil (all registered tools)
ReadOnly:      false
SystemPrompt:  (none — uses base system prompt only)
```

Full tool access. Default when no `-profile` flag is given. Use for unrestricted coding assistance. The global base prompt still steers toward dedicated tools over raw shell (**D12**).

---

### explore

```
Go var:        agents.Explore
Tool allowlist: read_file, glob, grep, web_fetch, web_search, todo_write
ReadOnly:      true
SystemPrompt:  "You are a fast, read-only explorer. Never modify files."
```

Bash is excluded. Safe for reading unfamiliar or untrusted repositories. Use when exploring a codebase without any modification risk.

---

### plan

```
Go var:        agents.Plan
Tool allowlist: read_file, glob, grep, web_search, todo_write
ReadOnly:      true
SystemPrompt:  "You are a software architect. Produce a clear, step-by-step implementation plan
               the user can follow. If the task is self-contained or greenfield (e.g. build a
               small app from scratch), answer directly from general knowledge without calling
               web_search. Use read_file, glob, and grep when the plan must reflect this
               repository's layout or existing code. Use web_search only for external docs, API
               versions, or facts you are unsure about — not for generic how-to or brainstorming."
```

No bash, no web_fetch. Use when you want an architecture or implementation plan grounded in the actual codebase.

---

### verification

```
Go var:        agents.Verification
Tool allowlist: read_file, bash, todo_write
ReadOnly:      false
SystemPrompt:  "You are a verifier. Return only PASS or FAIL with a brief reason."
```

Can execute shell commands to run tests or check build output. Produces a verdict, not a discussion. Use at the end of a development phase to confirm correctness. Bash remains subject to allowlist + single-command syntax rules ([tool-contract.md](./tool-contract.md), [`bash.go`](../../goclaw/internal/tools/bash.go)).

---

### guide

```
Go var:        agents.Guide
Tool allowlist: [] (no tools)
ReadOnly:      true
SystemPrompt:  "You answer questions about this codebase. Never run commands."
```

Chat-only. No tools registered — the model answers from context and conversation history. Use for documentation Q&A or explaining code without any execution.

---

### statusline

```
Go var:        agents.StatusLine
Tool allowlist: [] (no tools)
ReadOnly:      true
SystemPrompt:  "Output a single short status line, no markdown."
```

Produces a single line of plain text. Use for status bar integration or terse progress indicators.

---

### coordinator

```
Go var:        agents.Coordinator
Tool allowlist: spawn_agent, stop_task, todo_write
ReadOnly:      true
SystemPrompt:  Delegation-first: break work into self-contained sub-tasks, call spawn_agent with full
               context (paths, constraints); use stop_task with task_id to cancel; synthesize worker
               results for the user; never use file or shell tools directly.
```

Hub-and-spoke orchestration: the model only delegates to **worker** runs (`spawn_agent`) with profiles `general-purpose`, `explore`, `plan`, or `verification`. Each worker has its own `session.Session` and cannot nest another coordinator. Implementation: [`goclaw/internal/coordinator`](../../goclaw/internal/coordinator/).

---

## Custom agents (Markdown)

goclaw loads optional definitions from `~/.goclaw/agents/*.md` and `.goclaw/agents/*.md` (YAML frontmatter + body). Same `name` as a built-in **replaces** the built-in for that process. Details: [`goclaw/internal/agents/profile.go`](../../goclaw/internal/agents/profile.go) loader + [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) (D19).

---

## Tool Filtering

The orchestrator applies profiles in `buildRequest()` inside [`goclaw/internal/orchestrator/orchestrator.go`](../../goclaw/internal/orchestrator/orchestrator.go):

1. If `ToolAllowlist` is non-nil, only tools whose names appear in the list are included in the tool specs sent to the LLM.
2. If `ReadOnly` is `true`, `bash` is removed from the effective tool list regardless of the allowlist.
3. If `ToolAllowlist` is an empty slice (`[]string{}`), no tools are offered (guide, statusline).
4. If `ToolAllowlist` is `nil`, all registered tools are offered (general-purpose).

This filtering is compile-time behavior applied at request build time — there is no runtime override per call.

---

## Permission Modes and Profiles

Permission modes (`ask`/`allow`/`deny`, configured in `tool_permissions`) are **orthogonal** to the profile's tool allowlist:

- A tool **not in the allowlist** is never offered to the LLM and cannot be called.
- A tool **in the allowlist** with mode `deny` is offered in the schema but always blocked when the LLM calls it — the orchestrator returns a `tool_result` with `is_error: true`.
- A tool **in the allowlist** with mode `ask` prompts the user on stderr before each call.

---

## Future / polish

> **Still open (not specific to profile table):**
> - **Team/Swarm** peer-agent topology (tmux-style, mailboxes) — out of scope for goclaw MVP; do not conflate with `coordinator`.
> - **Context attachment policy per profile:** optionally omit loading `CLAUDE.md` or `git status` in Explore/Plan to save tokens (today all profiles share the same system prefix pattern).
> - **Reference-only docs:** [CUSTOM_AGENTS.md](./custom-agents.md), [COORDINATOR_MODE.md](./coordinator-mode.md), [YOLO_CLASSIFIER.md](./yolo-classifier.md) may describe a larger product than the Go CLI; trust [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) for shipped behavior.

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: 6-type table, token lesson, Go mapping, phase alignment. |
| 2026-04-07 | Cross-links: memory extractor, COORDINATOR_MODE, YOLO_CLASSIFIER, CUSTOM_AGENTS. |
| 2026-04-08 | Updated goclaw section: real file paths; plan → execute workflow; D16 sketch link. |
| 2026-04-09 | Seven built-in profiles (`coordinator`); D16 + custom `.md` agents documented; Post-MVP → future/polish. |
| 2026-04-08 | Translated to English; restructured around `profile.go` facts; reference-product analysis removed; exact system prompts included. |
| 2026-04-08 | Shared `baseSystemPrompt` (**D12**), verification/bash policy pointer to TOOL_CONTRACT and `bash.go`. |
| 2026-04-08 | `todo_write` on explore/plan/verification; plan → execute workflow (`.goclaw/plan.md`, `/apply-plan`); D16 sketch: [docs/goclaw/coordinator.md](../goclaw/coordinator.md). |
