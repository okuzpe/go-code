# Custom agents (Markdown + frontmatter) — reference and Go mapping

**Status in goclaw:** **D19 implemented** — Markdown + YAML frontmatter in `~/.goclaw/agents/*.md` and `.goclaw/agents/*.md`; see [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) and [`agent-profiles.md`](./agent-profiles.md).

Third-party analysis (Claude Code–style, **not** a feature checklist for goclaw): [Custom Agents — claude-code-explain](https://claude-code-explain.helmcode.com/custom-agents).

---

## Supported in goclaw (D19)

**Source of truth:** [`goclaw/internal/agents/loader.go`](../../goclaw/internal/agents/loader.go) (`customFrontmatter` struct).

| YAML key | Role |
|----------|------|
| `name` | Profile id (`[a-z0-9-]+`); must be unique after merge |
| `description` | Parsed and stored on the profile (diagnostics / future UX; not a hosted “subagent picker”) |
| `model` | Optional per-profile Ollama model override |
| `tool_allowlist` | Allowed tool names; **omit or YAML null** → all tools (`nil` in code); **`[]`** → no tools |
| `disallowed_tools` | Tool names removed from the effective set when the profile is built |
| `read_only` | When true, write tools and `mcp__` tools are not available |
| `system_prompt` | Optional system text from YAML (in addition to the Markdown body) |
| `max_turns` | Max orchestrator iterations for this profile (`0` = built-in default **32**) |
| `memory` | `user`, `project`, or `local` — per-agent memory root under `~/.goclaw/` / `.goclaw/` (see [memory-system.md](./memory-system.md), D13/D19 in CLAUDE.md) |

**Markdown body** (after the closing `---` of the frontmatter): appended to the profile’s system prompt.

**Discovery:** `~/.goclaw/agents/*.md` and `<workspace>/.goclaw/agents/*.md`; **project overrides user** for the same `name`; **hot-reload** when switching profile via `/profile`.

**Not implemented in goclaw’s frontmatter parser:** `hooks`, `mcpServers`, `skills`, `permissionMode`, `color`, `isolation`, enterprise priority chains, `--agents` JSON, or “Agent tool” invocation — those appear only in the **reference** sections below or in other products.

---

## Reference material (upstream patterns; partial overlap)

The sections below mix **goclaw-relevant links** with **design notes** from other agent stacks. If a capability is not listed under **Supported in goclaw** above, assume it is **not** in `loader.go` unless [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) explicitly ships it.

---

## 1. Where it fits in our documents

| Document | Relation |
|----------|----------|
| [agent-profiles.md](./agent-profiles.md) §2 | The **8 built-in profiles** (incl. `coordinator`, `code-review`) are the built-in set; a custom with the **same name** can **override** the built-in (project over user over built-in). |
| [coordinator-mode.md](./coordinator-mode.md) | goclaw uses in-process **`spawn_agent`** / **`stop_task`** and profiles — not a hosted **“Agent” tool** API; see [coordinator.md](../goclaw/coordinator.md). |
| [hooks.md](./hooks.md) | Project/user **hooks** use `external_hooks` and `.goclaw/hooks.json` — **not** YAML `hooks` in agent Markdown (reference-product pattern only). |
| [memory-system.md](./memory-system.md) | **Project** memory (`MEMORY.md`) ≠ **per-agent** memory (`memory: user|project|local`); see §5 of this doc. |
| **MCP / skills** | Runtime MCP and SKILL.md injection are configured via **settings** and discovery paths in CLAUDE.md — **not** via `mcpServers` / `skills` keys in agent frontmatter. |
| [yolo-classifier.md](./yolo-classifier.md) | **D17** risk scoring + `yolo_threshold` in settings; not a per-agent `permissionMode` field in Markdown. |

---

## 2. Definition types and priority (reference)

| Type | Typical origin |
|------|----------------|
| **Built-in** | Code (dynamic) — table in [agent-profiles.md §2](./agent-profiles.md) |
| **Custom** | `agents/*.md` in user / project paths |
| **Plugin** | `plugin/agents/*.md` + security restrictions (§7) |
| **CLI flag** | `--agents` JSON, session-only |

**Priority order** (highest wins in reference): managed enterprise → session flag → **project** `agents/` → **user** `~/…/agents/` → **plugin** → **built-in** (lowest).

**Go mapping (reference):** other products use `Resolve(name, sources...)`-style tables; goclaw merges **built-in** + **user dir** + **project dir** in [`loader.go`](../../goclaw/internal/agents/loader.go) / `AllWithCustom` — there is **no** `GOCLAW_SIMPLE` skip flag unless added explicitly in code.

---

## 3. File format (conceptual)

- **Illustrative paths (ref.):** `<cwd>/.claude/agents/foo.md`, `~/.claude/agents/foo.md`.
- **Go mapping:** `.goclaw/agents/*.md` and `~/.goclaw/agents/*.md` (exact names in **D19** + D7).

**Frontmatter — key fields**

| Field | Role |
|-------|------|
| `name` | Identifier → `subagent_type` |
| `description` | **Critical for selection:** concrete "Use when…"; multiline with `\n` |
| `tools` / `disallowedTools` | Allowlist / denylist (see §4) |
| `model`, `effort` | Override or `inherit` |
| `permissionMode` | default, acceptEdits, bypassPermissions, dontAsk, plan, auto |
| `color` | UI identity (fixed palettes in ref.) |
| `maxTurns`, `background` | Limits and background execution |
| `memory` | `user` \| `project` \| `local` — see §5 |
| `isolation` | e.g. `worktree` (isolated git, auto-cleanup) |
| `hooks` | Same schema as [hooks.md](./hooks.md); scoped to agent session |
| `mcpServers` | Named references or inline HTTP/stdio |
| `skills` | Names to preload |

**Body:** system prompt after the second `---`; if empty → generic default prompt (in reference).

---

## 4. Tool resolution (reference)

1. `tools` absent → full set.  
2. `tools: []` → none.  
3. `tools: ["*"]` → all.  
4. Explicit list → only those.  
5. `disallowedTools` applies on top of the allowlist.  
6. If `memory` active + explicit list → in ref. Read/Write/Edit are **injected** to manage agent memory.  
7. Agent MCP tools are **merged**.  
8. Async agents can remove tools like UserInput.

**Go mapping:** `agentprofile.ApplyToRegistry(base Registry) (*Registry, error)` consistent with `internal/tools`.

---

## 5. Per-agent memory vs MEMORY.md

Three **scopes** in reference (directories under `.claude/` in the analysis):

| Scope | Typical location | Notes |
|-------|-----------------|-------|
| `user` | `~/.claude/agent-memory/<name>/` | No required VCS |
| `project` | `<cwd>/.claude/agent-memory/<name>/` | Can be versioned |
| `local` | `…/agent-memory-local/<name>/` | No VCS |

Flow: create dir if missing → load `MEMORY.md`-style index → add scope guidelines to prompt.

**Snapshots:** team can commit a baseline in `agent-memory-snapshots/` to hydrate new agents.

**Go mapping (shipped):** YAML `memory: user|project|local` → `agents.Profile.MemoryScope` → `memory.PerAgentMemoryDir` → `memory.New(dir)` for the active session store in [`chat_wiring.go`](../../goclaw/internal/app/chat_wiring.go) and worker stores in [`spawn_agent.go`](../../goclaw/internal/coordinator/spawn_agent.go). Paths: `~/.goclaw/agent-memory/<name>/`, `<cwd>/.goclaw/agent-memory/<name>/`, `<cwd>/.goclaw/agent-memory-local/<name>/` respectively (**D19**).

---

## 6. Invocation (reference product vs goclaw)

**Reference:** an “Agent” tool with fields such as `subagent_type`, `model`, `run_in_background`, `isolation`, `cwd`.

**goclaw:** workers are started with the **`spawn_agent`** tool (coordinator profile): task string + **worker profile name** (built-in or custom from the merged map). See [coordinator.md](../goclaw/coordinator.md).

---

## 7. Agents in plugins (restrictions, reference)

Packaging overview: [plugins.md](./plugins.md). In the analyzed product's restrictive mode, plugin-defined agents **cannot**: escalate `permissionMode`, register arbitrary custom **hooks**, or declare their own **mcpServers**. Name uses `plugin:agent` namespace.

**Go mapping:** plugin `TrustLevel` + validation in loader.

---

## 8. Not in `settings.json` (reference)

Agents are **not** declared inside the settings schema; they are files or `--agents` JSON.

---

## 9. `/agents` (full product)

Interactive UI: list by source, view details, create assistant (wizard), edit/delete user/project agents. **goclaw today:** loads Markdown agents from disk (`~/.goclaw/agents/`, `.goclaw/agents/`), `/profile`, hot-reload — **without** the `/agents` UI of the reference product.

---

## 10. Go mapping (summary)

| Piece | Suggested location |
|-------|-------------------|
| Discovery and priority merge | `internal/agents` (`discover.go`, `resolve.go`) |
| `CustomAgentDef` type | Parse frontmatter + body; tests with `testdata/agents/*.md` |
| System prompt construction | `internal/orchestrator` layers: body → agent memory → env/CWD → optional CLAUDE.md |
| Per-agent hooks | Delegated to `internal/hooks` with `agentID` scope |
| Worktree isolation | `internal/tools/git` or wrapper; **D19** flag |

**Status:** custom agents on disk + merge order + **`memory` / `tool_allowlist` / `max_turns`** match D19 in [roadmap.md](../goclaw/roadmap.md) Tier 6 and [CLAUDE.md](../../goclaw/CLAUDE.md). **Frontmatter `hooks` / `mcpServers` / `skills`** are **not** implemented; the interactive `/agents` UI from the reference product is **not implemented**.

---

## 11. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: types, priority, frontmatter, tools, agent memory, MCP/hooks, plugins, Go mapping, helmcode §20 link |
| 2026-04-12 | Translated from Spanish to English |
| 2026-04-14 | **Supported in goclaw** table (`loader.go`); reference sections labeled non-implemented frontmatter features |
