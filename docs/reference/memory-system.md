# Persistent memory across sessions — reference and Go mapping

**Status in goclaw:** Filesystem store under `~/.goclaw/memory/` with `MEMORY.md` index; REPL `/memory` — see **D13** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). This doc adds **reference-product** depth.

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (D13 memory). Conceptual reference (third-party): [Memory — claude-code-explain](https://claude-code-explain.helmcode.com/memory).

### How to read this file

| Label | Meaning |
|-------|---------|
| **Shipped** | Matches current `goclaw` behavior (`internal/memory`, orchestrator wiring) unless a sentence is explicitly marked otherwise. |
| **Reference** | Upstream-style calibration (limits, hygiene); use for judgment, not as a full product spec. |
| **Proposal** | Suggested follow-up UX or implementation; **not shipped** unless the paragraph says it is. |

---

## 1. Is it worth including? (**Shipped** core, **Reference** depth)

**Yes**, and **it is already in goclaw** (D13): history in RAM (`internal/session`) does not survive process exit; disk memory captures **stable facts about the user, project, and feedback** without filling the prompt with git or duplicated code. The rest of this doc covers the **reference product** pattern for calibrating limits and UX.

**Not the same as:**

| Concept | Role |
|---------|------|
| `session` | Turns of the **current** conversation; compaction / [context-compaction.md](./context-compaction.md) |
| `CLAUDE.md` | Repo conventions, versioned with the code |
| **Memory** (this doc) | Facts **opaque to the code**: user role, validated corrections, deadlines, pointers to Linear/Slack, etc. |
| **Per-agent memory** (`memory: user/project/local` in frontmatter) | Dedicated directories per agent + `MEMORY.md`-style index; different scope from the global index — [custom-agents.md §5](./custom-agents.md). |

---

## 2. Four types (reference taxonomy) (**Reference** taxonomy, **Shipped** types in D13)

| Type | Typical content | When to save (heuristic) |
|------|-----------------|--------------------------|
| **user** | Role, preferences, level (e.g. "data scientist, new to React") | When learning the user's work style or goals |
| **feedback** | Corrections and **validated** approaches (e.g. "don't mock the DB in tests") | After explicit correction or confirmation that something worked |
| **project** | Work in progress, dates, incidents (e.g. "merge freeze 2026-03-05") | When clarifying who, what, when; convert relative dates to absolute |
| **reference** | Pointers to external systems (e.g. "pipeline bugs in Linear INGEST") | When citing external tools/channels/projects |

Each entry can be a **Markdown** file with **YAML frontmatter** (type, date, title); the index aggregates references.

---

## 3. `MEMORY.md` index and hard limits (**Reference** ceilings; **Proposal** for user-visible truncation warnings)

In the reference product the index:

- Is always loaded into context (dynamic layer of the system prompt).
- Has a **~200-line** and **~25 KB** ceiling; if exceeded, it is **truncated** (in the reference product, without an explicit user warning).

**Product tip (reference):** [practical-tips.md §8](./practical-tips.md) — reinforce that the index contains **only pointers** and consider **warning** before truncation.

**Design mapping:**

- Implement the same limits with **byte and line measurement** before injecting into the prompt.
- Possible UX improvement: **warn** (log or TUI) when the index exceeds the threshold and the user should condense manually.
- Index entries should be **short** (e.g. ~150 characters per child file), one line per file.

---

## 4. File structure (proposal for our CLI) (**Shipped** `~/.goclaw/memory/` layout; **Reference** alternate layouts)

We are not required to use `~/.claude/`; a dedicated namespace is better, e.g.:

```
~/.goclaw/memory/
├── MEMORY.md              # index (limits as in §3)
├── user_role.md
├── feedback_testing.md
└── reference_tools.md
```

Or per project in the repo: `.goclaw/memory/` (decision **D7** / **D14**). The **slug** can be a hash of the repo path or a configured name.

---

## 5. What should **not** go in memory (**Reference** hygiene)

Avoid contamination and staleness:

- Code patterns or paths derivable from the repo (that belongs in project docs or the code itself).
- Git history or authors (use `git log` / `git blame`).
- Debug "recipes" whose result is already in the code.
- What already lives in `CLAUDE.md` (duplicating splits the truth).
- Ephemeral details of the current session.
- PR lists or activity summaries (age poorly).

---

## 6. Automatic extraction (advanced phase) (**Shipped** `memory_llm_silent_extract`; **Reference** table for background pattern)

Pattern from the reference product (summary):

| Aspect | Behavior |
|--------|----------|
| Execution | **Background**: fork / sub-agent without blocking the main session |
| Trigger | After turns **without** tool calls ("silent turns") |
| Budget | Max **~5 turns** for the extractor agent |
| Permissions | Broad Read/Grep/Glob; Bash **read-only**; Write/Edit **only** under `memory/` |
| Dedup | If the main agent already wrote memory in the session, skip re-extraction |
| Cache | Share prompt cache with the parent session when the provider allows it |

**Go mapping (goclaw):** `memory.ScheduleSilentTurnLLMExtract` — goroutine + 2-minute `context` timeout; one non-tool LLM JSON reply; writes via `memory.Store.Save` into the active store (global, per-agent, or worker store). Opt-in settings key **`memory_llm_silent_extract`** (default off). Trigger: end of a user turn where **no** tools ran and the user message is not a `/` slash command.

---

## 7. Go package mapping (**Shipped**)

| Piece | Package |
|-------|---------|
| Load index + fragments for the prompt | `internal/memory` + call from `internal/orchestrator` |
| `memory_read` / `memory_write` tools (optional) | `internal/tools`, with **permissions** that restrict paths to the memory tree |
| Background silent-turn extractor | [`internal/memory/extractor.go`](../../goclaw/internal/memory/extractor.go) (scheduled from `internal/orchestrator` after tool-free turns) |

Dependencies: `orchestrator` → `memory`; `memory` **does not** import `orchestrator`.

---

## 8. Changelog (**Meta**)

| Date | Change |
|------|--------|
| 2026-04-07 | Created from the Claude Code model (third-party explainer); limits, 4 types, anti-patterns, extractor, own paths |
| 2026-04-07 | Explicit link to [context-compaction.md](./context-compaction.md) in §1 table (`session`) |
| 2026-04-07 | §1: per-agent memory vs. global index → [custom-agents.md](./custom-agents.md) §5 |
| 2026-04-07 | §3: link [practical-tips.md §8](./practical-tips.md) (index truncation) |
| 2026-04-12 | Translated from Spanish to English |
| 2026-04-12 | §6–§7: shipped silent-turn extractor (`memory_llm_silent_extract`) in `internal/memory/extractor.go` |
| 2026-04-17 | Reading guide table (**Shipped** / **Reference** / **Proposal**); section headings tagged for scope |
