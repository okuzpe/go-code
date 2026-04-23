# Coordinator Mode and Team/Swarm — reference and Go mapping

**Status in goclaw:** **Coordinator (hub-and-spoke)** is implemented — [`docs/goclaw/coordinator.md`](../goclaw/coordinator.md), `internal/coordinator`, `--profile coordinator`. **Team/Swarm** peer topology here remains reference-only.

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (D16 coordinator). Conceptual reference (third-party analysis of Claude Code): [Coordinator Mode — claude-code-explain](https://claude-code-explain.helmcode.com/coordinator-mode).

**Key message:** the analyzed product has **two distinct multi-agent systems**, not one system with two names. Conflating them in your own design produces incoherent permissions (e.g. a "coordinator" that can still `Write`).

**goclaw (shipped):** multi-agent delegation is the **hub-and-spoke coordinator** — `--profile coordinator`, tools **`spawn_agent`** and **`stop_task`**, worker profiles by name (built-in or custom). It does **not** expose the reference product’s **Agent** tool or `team_name` peer mesh. See [`docs/goclaw/coordinator.md`](../goclaw/coordinator.md) and D16 in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). [`docs/goclaw/swarm.md`](../goclaw/swarm.md) is reference-only in this checkout.

---

## 1. Two topologies

| Aspect | **Coordinator Mode** (hub-and-spoke) | **Team / Swarm** (peer-to-peer) |
|--------|--------------------------------------|----------------------------------|
| Topology | One central **coordinator**; only it talks to workers | Any peer can message any other |
| Communication | Coordinator ↔ workers | Many-to-many |
| Terminal UI | No special UI in reference | **tmux** (splits, colors); alternative **iTerm2** (AppleScript) on macOS |
| Activation (ref.) | Build flag + env `CLAUDE_CODE_COORDINATOR_MODE=1` | **Agent** tool with **`team_name`** parameter |
| Best fit (ref.) | Complex tasks with **strict** orchestration | Parallel collaborative work |

---

## 2. Coordinator Mode (detail)

### 2.1 Key security restriction

The **coordinator does not access the filesystem or shell**: no direct Read/Write/Edit/Bash. All mutations or code reads go through **workers** that have the full toolbox. It is a **reasoning and delegation** layer with no "I accidentally edited the repo" surface.

### 2.2 Coordinator tools (reference)

Only a subset of **~4 tools** oriented toward orchestration, e.g.:

- **Agent** — launch / route workers  
- **SendMessage** — continue a worker by id or send instructions  
- **TaskStop** — stop tasks  
- **SyntheticOutput** — structured output when the product requires it  

(Workers retain the full set minus whatever the product blocks by policy; in the reference product the coordinator is heavily restricted.)

### 2.3 Other changes vs. normal mode

| Aspect | Normal mode | Coordinator mode (ref.) |
|--------|-------------|-------------------------|
| System prompt | Standard prompt | **Coordinator-specific** prompt |
| File / shell access | Direct via tools | **Only via workers** |

### 2.4 Four-phase workflow (reference)

| Phase | Who | Goal |
|-------|-----|------|
| 1. Research | Workers (**parallel**) | Explore code / context |
| 2. Synthesis | Coordinator | Read findings; **draft spec** with concrete paths and **line numbers** |
| 3. Implementation | Workers (**sequential** per file area) | Apply changes per spec; one worker at a time per zone |
| 4. Verification | Workers | Confirm it works; can overlap with impl in **different areas** |

### 2.5 Worker result → coordinator (reference format)

Results come back as **user-role** messages with encapsulated XML, e.g.:

- `task-id`, `status` (`completed` | `failed` | `killed`)
- `summary`, `result` (detailed output), `usage` (tokens or metrics)

**Go mapping:** `WorkerNotification` type with stable XML or JSON; the orchestrator parses and **rewrites** internal state without passing the raw worker transcript to the next coordinator turn if policy requires it.

### 2.6 Continue vs. spawn (reference heuristic)

| Situation | Action | Reason |
|-----------|--------|--------|
| Worker already explored files that need editing | **Continue** (`SendMessage` to same id) | Preserves loaded context |
| Wide research but narrow implementation | **Spawn** new | Don't carry unnecessary context |
| Fixing a failure | **Continue** | Keeps the error in context |
| Verifying another worker's code | **Spawn** new | Fresh eyes |
| Previous approach was wrong | **Spawn** new | Clean restart |

### 2.7 Critical rule: worker prompt isolation

Workers **do not see** the coordinator's conversation with the user. Each worker prompt must be **self-contained**: the coordinator must include **exact paths, line numbers, and instructions** in the delegation. Never rely on phrases like "based on your earlier findings" — the worker does not have those findings unless they are in **its** task message.

**Matches [agent-profiles.md §3](./agent-profiles.md)** (inject into delegation vs. loading full `CLAUDE.md`): same philosophy of **minimal but sufficient prompt**.

---

## 3. Team / Swarm (detail)

### 3.1 Disk structure (reference, illustrative paths under `~/.claude/`)

| Artifact | Typical path (ref.) |
|----------|---------------------|
| Team config | `teams/{team-name}/config.json` |
| Task list | `tasks/{team-name}/` |
| Mailboxes (inboxes) | `teams/{team-name}/inboxes/{name}.json` |
| Fixed "team lead" role | Reserved name e.g. `team-lead` |

### 3.2 Summarized flow

1. Team lead creates team (**TeamCreate**).  
2. Creates tasks (**Task*** tools).  
3. Launches peers (**Agent** with `team_name` + name).  
4. Assigns owners (**TaskUpdate**, owner field).  
5. Peers work and close tasks.  
6. Messaging via **SendMessage** (peer-to-peer).  
7. Shutdown: **SendMessage** with type `shutdown_request` / response.  
8. Cleanup **TeamDelete** when no active peers remain.

### 3.3 Terminal backends (reference)

| Backend | Notes |
|---------|-------|
| **In-process** | Isolated with AsyncLocalStorage (in TS); for tests / headless |
| **tmux** | Separate Claude Code process per pane; splits and titles |
| **iTerm2** | macOS, AppleScript for splits/tabs |

Layouts (ref.): inside existing tmux → lead ~30% left, team ~70% right (`main-vertical`); outside tmux → new session `claude-swarm`, window `swarm-view`, isolated socket, `tiled` layout.

### 3.4 Dual SendMessage (reference)

- **Coordinator Mode:** `SendMessage({ to: "<agent-id>", message: "..." })` continues a worker.  
- **Team/Swarm:** message to a peer by name, or **broadcast** `to: "*"` (use sparingly).

**Routing (ref.):** prefixes like `bridge:<id>`, `uds:<path>`, registered agent id, `*` for mailbox broadcast, or name → write to file mailbox.

### 3.5 Mailboxes (JSON files + locking)

- JSON messages: `from`, `text`, `timestamp`, `read`, optional `color`, `summary`.  
- **proper-lockfile** (in reference) to prevent write races.  
- Poll ~**1 s** (React UI hook in reference).  
- The poll can also carry permissions, sandbox, shutdown, plan approval, etc.

Injected into the peer's context as XML, e.g. `<teammate-message teammate_id="..." ...>`.

---

## 4. Coordinator vs. Team/Swarm summary table

| Aspect | Coordinator | Team/Swarm |
|--------|-------------|------------|
| Topology | Hub-and-spoke | Peer-to-peer |
| Messages | Coordinator ↔ workers | Anyone ↔ anyone |
| Tools for "center" | Orchestration only | Team lead with broad set (ref.) |
| Workers | Full toolbox (no Agent/SendMessage outward per policy) | Toolbox + **SendMessage** between peers |
| Work delivery | XML `task-notification` | Mailboxes + XML teammate-message |
| UI | No forced tmux | tmux / iTerm2 |

---

## 5. Go mapping (without copying the TS stack)

| Reference piece | Go proposal |
|-----------------|-------------|
| Activating flag + env | `internal/config`: `CoordinatorMode bool`, validated at startup |
| Role tool profile | [agent-profiles.md](./agent-profiles.md): **Coordinator** = explicit allowlist; **Worker** = General-Purpose or specialized |
| Isolated workers | Child process **or** goroutine + `session` fork per `agent_id`; **never** share the coordinator's message slice |
| SendMessage / TaskStop | `internal/tools` + `orchestrator` routing to registry of live workers |
| Team/Swarm mailboxes | Future/reference mapping only: `internal/swarm` or `internal/team`: `Mailbox interface { Push, Poll }` with **dir+lock** implementation first; optional Redis/DB later |
| tmux/iTerm2 | **No parity** with the reference product's tmux/iTerm; on Windows prioritize **headless** + logs |
| 1 s polling | `time.Ticker` or events with `fsnotify` where it makes sense |

**Dependencies (aligned with [CLAUDE.md](../../goclaw/CLAUDE.md)):** `coordinator` / `swarm` must not import `tools` in a circular way; the **orchestrator** owns the `agentID → run context` map.

---

## 6. Reference product phases vs. goclaw

| Phase (reference) | What it meant |
|-------------------|----------------|
| Product without multi-agent | Single REPL loop |
| Coordinator introduced | Allowlist + isolated workers (no required tmux) |
| Team/Swarm | Peer mailboxes, shared tasks, optional UI |

**goclaw today:** **D16 hub-and-spoke** (`spawn_agent` / `stop_task`, `coordinator` profile) is **implemented** — see [coordinator.md](../goclaw/coordinator.md). **Team/Swarm** peer-style remains **reference design** in this checkout; there is no shipped `internal/swarm` package here.

---

## 7. Relation to other docs

- **[context-compaction.md](./context-compaction.md):** each worker has its own context budget; the coordinator compacts **its** thread, not the worker's internal one.  
- **[memory-system.md](./memory-system.md):** stable decisions after a multi-agent session may deserve `project` / `feedback` entries; they do not replace specs in XML.  
- **§2.1 dedicated tools:** workers still follow D12; the coordinator does not use Bash to read the repo.
- **[yolo-classifier.md](./yolo-classifier.md):** in auto mode, workers with the full toolbox must pass the same **security gate** as a simple session; delegation does not substitute the classifier.

---

## 8. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: two topologies, phases, mailboxes, continue/spawn, worker-does-not-see-coordinator invariant, Go mapping, helmcode §16 link |
| 2026-04-12 | Translated from Spanish to English |
