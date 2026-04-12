# Custom agents (Markdown + frontmatter) — reference and Go mapping

**Status in goclaw:** **D19 implemented** — Markdown + YAML frontmatter in `~/.goclaw/agents/*.md` and `.goclaw/agents/*.md`; see [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) and [`agent-profiles.md`](./agent-profiles.md).

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (D19 custom agents) and [agent-profiles.md](./agent-profiles.md). Reference (third-party, Claude Code analysis): [Custom Agents — claude-code-explain](https://claude-code-explain.helmcode.com/custom-agents).

**Idea:** one **.md per agent** with **YAML frontmatter** that fixes the operational identity (tools, model, permissions, MCP, hooks, memory, color); the **Markdown body** is the system prompt. No extra code to "register" the agent beyond placing it in a discovery path.

---

## 1. Where it fits in our documents

| Document | Relation |
|----------|----------|
| [agent-profiles.md](./agent-profiles.md) §2 | The **7 built-in profiles** (incl. `coordinator`) are the built-in set; a custom with the **same name** can **override** the built-in in the reference (priority). |
| [coordinator-mode.md](./coordinator-mode.md) | The **Agent** tool picks `subagent_type` → resolves custom or built-in definition. |
| [hooks.md](./hooks.md) | Frontmatter **`hooks`**: registers **session** hooks when the sub-agent spawns; cleaned up on finish; `Stop` → `SubagentStop` in reference. |
| [memory-system.md](./memory-system.md) | **Project** memory (`MEMORY.md`) ≠ **per-agent** memory (`memory: user|project|local` + dedicated directory); see §5 of this doc. |
| §2.8 **MCP** | `mcpServers` in frontmatter: named references or inline definition; cleanup on agent finish if applicable. |
| §2.9 **Skills** | `skills` field to preload content before the first turn. |
| [yolo-classifier.md](./yolo-classifier.md) | Agent `permissionMode` limits or expands risk; still passes through **D17** in auto mode. |

---

## 2. Definition types and priority (reference)

| Type | Typical origin |
|------|----------------|
| **Built-in** | Code (dynamic) — table in [agent-profiles.md §2](./agent-profiles.md) |
| **Custom** | `agents/*.md` in user / project paths |
| **Plugin** | `plugin/agents/*.md` + security restrictions (§7) |
| **CLI flag** | `--agents` JSON, session-only |

**Priority order** (highest wins in reference): managed enterprise → session flag → **project** `agents/` → **user** `~/…/agents/` → **plugin** → **built-in** (lowest).

**Go mapping:** explicit table in `internal/agents` (`Resolve(name string, sources ...)`); env flag like `GOCLAW_SIMPLE=true` can **skip** customs (equivalent to `CLAUDE_CODE_SIMPLE`).

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

## 6. Invocation (Agent tool)

Conceptual fields: `description`, `prompt`, `subagent_type`, `model`, `run_in_background`, `name`, `team_name`, `mode`, `isolation`, `cwd` (advanced contexts).

**Fork vs fresh (reference):** without `subagent_type` can inherit parent context; with a defined type → own prompt/tools and fresh window.

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

**Status:** custom agents + priority merge + hooks/MCP in frontmatter are **aligned** with D19 in [roadmap.md](../goclaw/roadmap.md) Tier 6 and [CLAUDE.md](../../goclaw/CLAUDE.md); the interactive `/agents` UI from the reference product is **not implemented**.

---

## 11. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: types, priority, frontmatter, tools, agent memory, MCP/hooks, plugins, Go mapping, helmcode §20 link |
| 2026-04-12 | Translated from Spanish to English |
