# goclaw — Usage

Task-oriented guide: how to run the CLI, configure providers, and use sessions, tools, and profiles. Deep reference for tools, limits, and architecture lives in [CLAUDE.md](../../goclaw/CLAUDE.md). Documentation: [README.md](../../goclaw/README.md), [docs-map.md](../docs-map.md).

## Quick start (Ollama)

Prerequisite: Ollama on `http://localhost:11434` with models pulled for your `ollama_model` and any **`task_models`** tags (built-in defaults: `qwen2.5-coder:7b`, `qwen2.5-coder:3b`, `qwen3:4b`, `qwen3:8b` — see [model-routing.md](./model-routing.md)).

```bash
cd goclaw
go run ./cmd/goclaw doctor
go run ./cmd/goclaw              # fullscreen TUI on TTY (default)
printf 'ping\n' | go run ./cmd/goclaw --mock --no-tools --output-format json   # pipes / CI (no TTY REPL)
```

Golden path inside the TUI:

1. start in **`build`** for direct coding
2. switch to **`plan`** when you want a read-only implementation plan first
3. run `/plan run` or `/apply-plan` when you are ready to execute
4. use `/continue` to keep the same task moving

### Local-first agent (sessions, tools, Ollama)

- **Primary binary:** `go run ./cmd/goclaw` — fullscreen chat, doctor, sessions, tools, and plan/apply flow.
- **Sessions** (roles, tool traces, plain export) persist as JSONL under `~/.goclaw/sessions/<id>.jsonl` on exit or `/save` (see [Sessions and memory](#sessions-and-memory) below).
- **Ollama:** set `OLLAMA_HOST` / `OLLAMA_MODEL` or `ollama_host` / `ollama_model` in `~/.goclaw/settings.json`. Use a **tools-capable** model (defaults target `qwen2.5-coder`); if the daemon rejects wire tools, the idle footer shows `Ollama text-only (no tool wire · /doctor)` — run **`/doctor`** or `goclaw doctor` for remediation. Raise **`ollama_num_ctx`** if context feels tight (see [Quick start](#quick-start-ollama) and compaction notes below).
- **Interact mode (Ctrl+M):** cycles **chat · code · agent** in the footer. This now steers the orchestrator for that send (system hint + lower iteration cap in **chat** mode). It is not the same as **`agent_profile`** (`/profile`, `agent_profile` in settings).
- **Tool output in the TUI:** **Ctrl+T** opens tool history; in **Ctrl+B** transcript browse, **[** / **]** focus a tool card and **e** toggles an expanded raw body in the transcript (large outputs are still capped; use Ctrl+T for the full step log).

### Large repo analysis and refactors

- **`.goclaw/plan.md`** is a local scratchpad; it may contain notes unrelated to goclaw. For product intent and architecture, prefer **[CLAUDE.md](../../goclaw/CLAUDE.md)**, **[README.md](../../goclaw/README.md)**, and the topic files under **[docs/goclaw/](./)** (see [docs-map.md](../docs-map.md)).
- For **“audit everything”** or whole-tree refactors: ask for **one slice per turn** (one package, one `internal/` area, or one doc). Check results with **`git diff`**, tool output, or successful write tools — not with narrative alone.
- **Disk changes** only happen when **`write_file`**, **`edit_file`**, or **`patch`** complete successfully (subject to permissions). If the assistant says it “applied changes” but no write tool ran, treat that as prose, not git truth.
- **Session undo for file writes** — successful `write_file`, `edit_file`, and `patch` operations now record a session-local checkpoint in the scratch directory. Use **`/undo`** to revert the latest captured file change from the current session.
- **Why it stopped at "Done"** — Each of your messages runs until the model answers **without** requesting more tools. A text-only reply (even "Done") **ends that turn**; there is no hidden auto-loop. For multi-step refactors, either write one prompt that forces **read → edit → verify** in one go, or send a **follow-up** ("continue: apply the first edit to `internal/...`").
- **Auto-continue (default on)** — When your message clearly asks for fixes/refactors and the model answers with **prose only** — including the **first** completion with **zero** tool calls (common with small local models) **or** after **read-only** tools only — goclaw may inject a short `[goclaw]` user line and **re-prompt the model** (default **2** times per user message for both patterns combined). Optional **`auto_continue_action_max_nudges`** in `settings.json` raises that cap (**1–5**; values above **5** are clamped). Set `"auto_continue_action_requests": false` to disable entirely. If nudges exhaust and tools still never run, see [Troubleshooting — Assistant explains plans but does not modify files](#assistant-explains-plans-but-does-not-modify-files).
- **Truth-on-disk footer (default on)** — If your message signals code changes, tools ran, writes are allowed for the profile, but no `write_file` / `edit_file` / `patch` completed successfully, the runtime may append a short bilingual `[goclaw]` footer to the assistant reply (and session log) so prose cannot claim edits alone. Set `"truth_footer_no_workspace_writes": false` in `settings.json` to disable. **After successful workspace writes:** the **TUI** may show a **`goclaw: git diff --stat`** block when the workspace is a git repo (bounded runtime and output size). After the truth footer, the TUI shows an extra hint under the session footer (mentions **`/continue`** and short follow-ups).
- **Turn shape** — One user message runs the loop until the model returns **no** tool calls (then the turn ends). If it stopped after reads only, typing **continue**, slash **`/continue`**, or nudging the agent is normal; **`/continue`** sends a standard follow-up while keeping full session history (see `/help`).
- **Context window** — If **`ollama_num_ctx`** is set in settings and is **below 8192** (see `OllamaNumCtxBannerWarnBelow` in `internal/app/ux_constants.go`), the **non-TTY startup banner** (when printed) warns that long system prompts plus tool schemas may truncate. Raise `ollama_num_ctx` if turns feel “forgetful” or tools behave oddly.
- **Iteration budget** — Past halfway through the per-turn iteration limit, a `<system-reminder>` warns the model that budget is tight. If you asked for real edits and tools already ran without a successful write, that reminder nudges toward **finishing edits** instead of only wrapping up in prose.

### Project context and standing orders (iterative bootstrap)

Each session, goclaw builds a **project context** string from the tool workspace root and injects it into the system prompt (implementation: `buildProjectContext` in [`internal/app/chat_wiring.go`](../../goclaw/internal/app/chat_wiring.go)).

- **Manifest + workspace awareness** — goclaw starts with a compact workspace summary: detected stack manifest (`go.mod`, `package.json`, `Cargo.toml`, or `pyproject.toml`), the top of `README*`, a bounded **workspace layout** summary, a short **git workspace** summary when the root is a repository, a lightweight **repo map** for Go workspaces (selected files plus top-level symbols), and, when present, key docs such as **`docs/docs-map.md`**, **`docs/architecture.md`**, `docs/README.md`, or `docs/index.md`. This gives explore/plan-style turns architecture and repo-shape context without loading full repo rules.
- **`CLAUDE.md`** at the workspace root — only the **first N lines** are included (default **60**). Put the highest-priority standing rules at the **top** of the file. Override the cap with **`project_context_claude_md_lines`** in `~/.goclaw/settings.json` or project `.goclaw/settings.json` (values are clamped to **1..200**).
- **Standing orders file** — If **`.goclaw/STANDING_ORDERS.md`** exists under the workspace root, its contents are appended to the project context (after `README` / manifest snippets and `CLAUDE.md`). Alternatively, set **`project_context_standing_orders_path`** to any **workspace-relative** path (must not escape the workspace; absolute paths are rejected). Limit lines with **`project_context_standing_orders_max_lines`** (default **40**, clamped to **1..120**); the injected block is also capped by bytes so prompts stay bounded.

**Workflow tip:** For multi-step work, keep the loop simple: draft in **`plan`**, switch to **`build`** to apply changes, and verify with real write tools plus `git diff` or tests. Maintainer-oriented detail: **[CLAUDE.md](../../goclaw/CLAUDE.md)** - section *Iterative work and standing orders*.

### First-run setup (onboarding)

The first time you run **interactive** goclaw on a TTY and **`~/.goclaw/settings.json` does not exist**, a short wizard runs **before** the chat UI:

1. Security summary (optional full text is bundled; same content as [security.md](./security.md))
2. Workspace trust for the current directory (`trusted_workspace` in project `.goclaw/settings.json`)
3. **TUI appearance** preset (fullscreen mode only; change later with `/theme`)
4. **Primary mode** — direct coding (**`build`**) or read-only planning (**`plan`**); written to `agent_profile` in `~/.goclaw/settings.json`. **Esc** on this step goes back to appearance.
5. **Provider**: Ollama (local); host and model are written under `~/.goclaw/` as described below

**Files written:** `~/.goclaw/settings.json` (and `settings.local.json` if you enter an API key); project `.goclaw/settings.json` when you confirm trust.

**Environment:**

- `GOCLAW_NO_ONBOARDING=1` — skip the wizard (advanced; you still need safe usage practices — see [security.md](./security.md))
- `GOCLAW_ONBOARDING=1` — force the wizard even if `settings.json` already exists (useful for testing)

**`goclaw doctor` does not run onboarding** — it loads config and shows a health report: **Bubble Tea** fullscreen pager on a TTY (scroll with arrows / PgUp / PgDn; **q**, **Esc**, or **Ctrl+C** to exit), or plain **`fmt.Println`** when stdin/stdout are not both terminals. Run `doctor` for a quick check; run `goclaw` once to complete first-time setup.

The wizard runs in the **same fullscreen Bubble Tea stack** as the main app (when stdin/stdout are TTY). During setup you pick **`build`** or **`plan`**; afterward use `/mode` for the primary flow or `/profile` for advanced profiles — see [Agent profiles](#agent-profiles). If you never run the wizard, **`config.Default()`** still starts in **`build`** mode.

### Interactive chat (TTY)

- **Default on a TTY** — fullscreen Bubble Tea (`internal/ui/chat`): transcript, compact tool approval above the input, `/focus` hint in the footer. The ASCII startup banner is **not** printed to stdout (welcome panel + footer carry session context).
- **`GOCLAW_USE_TUI=0` on a TTY** is **unsupported** — there is no line-at-a-time REPL. Use a real terminal for interactive chat, or **`--output-format json`** / **`goclaw prompt`** for automation.

Exit: `Esc` (TUI) or `Ctrl+C`. Clear transcript: `Ctrl+L` or slash **`/clear`** (both clear inside the Bubble Tea model; no raw ANSI screen erase). Prior submit lines: **↑** / **↓** in the compose box (single-line recall; persisted under `~/.goclaw/history`).

### Slash commands, autocomplete, and help

- **Slash line** — Type `/` on a **single line** to see a **filtered list** of commands as you keep typing (prefix match). **Tab** completes the command (longest shared prefix, or the only match). After the command name, the strip shows **argument** suggestions where supported (e.g. `/profile`, `/memory`, `/plan`, `/resume`, `/focus`, `/theme`, `/export`); **Tab** completes the argument token at the cursor the same way.
- **Document overlay** — **`/help`** (also `help` / `?`) opens a dismissible panel with **Markdown** (rendered for the terminal, including shortcuts + slash reference). **`/capabilities`**, **`/doctor`**, **`/sessions`**, **`/memory list`**, bare **`/agents`**, **`/plan review`**, **`/plan steps`**, **`/apply-plan --preview`**, and **`/init`** use the same overlay when output is structured as a document. **Esc** closes the panel; **↑** / **↓** (or `k` / `j`) and **PgUp** / **PgDn** scroll; the panel **re-wraps** when you resize the terminal. **Ctrl+C** still quits the app from the panel.

### Prefix input (`!`, `@`, `&`, `/btw`)

Interpreted **after** slash commands and **before** the model (interactive TUI). Same **permission policy, approval, and hooks** as normal tool calls. **Single line** for `!` and `&` (extra lines are rejected). **`--mock`** disables prefix handling.

| Prefix | Meaning |
|--------|---------|
| `!` + command | Run the **`bash`** tool with that command (allowlist and metacharacter rules apply). |
| `@` + path (standalone) | Run **`read_file`** for a path inside the workspace. Matching paths appear under the input as you type; **Tab** completes anywhere in the line. Drag-and-drop a file/folder onto the terminal to insert `@relpath` automatically. |
| `@token` inline | When `@path` tokens appear inside a larger message (e.g. `explain @go.mod`), the file is silently pre-loaded before the model call — no separate read step needed. |
| `&` + task | Run **`spawn_agent`** with worker profile **`build`** (requires **`spawn_agent`** on the active profile — use **`/profile coordinator`** for hub mode if your current profile does not include it). |
| `/btw` + text | Slash command: submit **one** user message wrapped as a short “side question” to the model. |

Full grammar and security notes: [prefix-input-modes.md](./prefix-input-modes.md).

## Sessions and memory

Sessions save as JSONL under `~/.goclaw/sessions/<id>.jsonl` on exit or `/save`.

**Copy / export chat (plain text from the session, not the styled TUI):**

- **`/copy`** — copies the in-memory transcript (roles, text, tool calls/results) to the **system clipboard**. Very long sessions are truncated (see `/help`). If the clipboard fails (headless SSH, etc.), use **`/export`**.
- **`/export path.txt`** — writes the same plain text to **`path.txt`**. If the path is relative and a workspace is set, it is resolved under that workspace; use an absolute path to write elsewhere.

The fullscreen TUI keeps **normal terminal mouse behaviour** by default (wheel scrolling is **off** so click–drag selection works as your host terminal allows). Enable wheel-on-transcript with **`tui_mouse_scroll`**: `true` in settings or **`GOCLAW_TUI_MOUSE_SCROLL=1`**. Use **Ctrl+B** in the TUI to scroll the transcript with the keyboard without capturing the mouse. For “everything in the session”, prefer `/copy` or `/export` over selecting the screen.

**Idle footer detail** (message count, token estimate, profile, etc.) uses **`tui_footer_density`** in settings or **`GOCLAW_TUI_FOOTER_DENSITY`**: **`minimal`** (short line; adds token/compact hints only when the context window is nearly full), **`standard`** (default — message count, token estimate, compaction %, profile name), **`debug`** (legacy dense line including `ro:` / `hub:` / tool counts and task-role tags). Omit the key to keep **`standard`**.

**Profile rail + keyboard cycle** in the fullscreen TUI: **`tui_profile_cycle`** in settings is an ordered list of agent profile names (built-in + custom `*.md`). **Ctrl+Shift+←** and **Ctrl+Shift+→** move to the previous/next name in that list (skipped names are not in the cycle). A **single-line rail** above the transcript shows a lane glyph, **‹profile›** (or `<profile>` in ASCII icon mode), an optional short hint (e.g. `hub`), a dotted trace, and a muted **shift+ctrl** chord — all one row so the layout does not break. If **`tui_profile_cycle`** is omitted, the default order is **build → plan → explore → coordinator** (each entry is still skipped when that profile does not exist).

```bash
go run ./cmd/goclaw --list-sessions
go run ./cmd/goclaw sessions list
go run ./cmd/goclaw --session <id>
```

**Memory** (cross-session Markdown under `~/.goclaw/memory/`): types `user`, `feedback`, `project`, `reference`. Use `/memory list|add|delete` in the REPL.

## One-shot automation (`prompt` and JSON)

**Default `goclaw` / `goclaw chat` on a pipe** (non-interactive stdin or stdout) **exits with an error** unless you select JSON output — use **`--output-format json`** (or **`--json-output`**) or **`goclaw prompt`**. There is no line-at-a-time REPL for pipes.

No interactive REPL:

```bash
go run ./cmd/goclaw prompt "summarize internal/cli" --no-tools
printf 'status\n' | go run ./cmd/goclaw --output-format json --no-tools
go run ./cmd/goclaw prompt "status" --output-format json --no-tools
```

`--json-output` is stdin shorthand for `--output-format json`. Tools that would **ask** in the REPL need `"allow"` in `tool_permissions` for non-interactive runs, or use `--no-tools`.

**Mock regression bundle (no API key):** `make parity` or [scripts/MOCK_PARITY_HARNESS.md](../../goclaw/scripts/MOCK_PARITY_HARNESS.md).

## Legacy providers (`anthropic`, `openai_compatible`)

Cloud Anthropic and OpenAI-compatible HTTP providers are **not supported** in the CLI. Use `"provider": "ollama"` (or omit it), `ollama_model` / `OLLAMA_MODEL`, and remove `openai_*` keys from settings if present.

## Configuration

Merge order (later overrides earlier):

```
defaults → ~/.goclaw/settings.json → .goclaw/settings.json
        → ~/.goclaw/settings.local.json → .goclaw/settings.local.json
```

Do not commit `settings.local.json`.

**Common keys:** `provider` (must be `ollama` for a normal run; legacy values error at startup), `agent_profile`, `ollama_model`, `ollama_host`, `ollama_num_ctx`, `bash_timeout_sec`, `tool_permissions`, `auto_continue_action_requests`, `auto_continue_action_max_nudges`, `truth_footer_no_workspace_writes`, `agent_verify_after_write`, `parallel_tool_batch_max`, `plan_require_apply_approval`, `plan_apply_use_coordinator`, `agent_picker_hidden_profiles`, `mcp_servers` (stdio or HTTP; HTTP entries may set `bearer_token_file` for a static bearer token), `mcp_allow_remote_urls`, `trusted_workspace`, `external_hooks`, `plugin_dirs`, `plugin_allow`, `plugin_deny`, `memory_auto_extract`, `ide_bridge_mcp`. CLI: `--plugin-dir` (repeatable) appends plugin roots.

Example:

```json
{
  "provider": "ollama",
  "agent_profile": "explore",
  "ollama_model": "qwen2.5-coder:7b",
  "bash_timeout_sec": 120,
  "tool_permissions": {
    "read_file": "allow",
    "bash": "ask",
    "web_fetch": "ask",
    "mcp__demo__example_tool": "ask"
  },
  "mcp_servers": [{ "id": "demo", "command": "node", "args": ["path/to/server.js"] }],
  "trusted_workspace": false
}
```

Full environment variable list: [CLAUDE.md — Environment Variables](../../goclaw/CLAUDE.md).

## Editor integration (VS Code / Cursor pattern)

To attach a **local editor MCP server** (HTTP on loopback) without hand-editing `mcp_servers` every time:

1. Put a JSON lockfile under **`~/.goclaw/ide/*.json`** with `url` (and optional `headers`).
2. Set **`ide_bridge_mcp`: `true`** in merged settings.
3. Optionally set **`GOCLAW_IDE_NOTIFY_URL`** to a loopback URL for post-tool POST pings.

**Step-by-step:** [ide-editor-setup.md](./ide-editor-setup.md) · **Contract:** [ide-bridge.md](../reference/ide-bridge.md) §6–§7 · **Example JSON:** [examples/ide-mcp-endpoint.example.json](./examples/ide-mcp-endpoint.example.json).

## Agent profiles

**Primary flow:** use **`/mode build`** for direct coding and **`/mode plan`** for read-only planning first.

**Advanced flow:** use `--profile <name>`, `agent_profile` in settings, or **`GOCLAW_AGENT_PROFILE`** when you want a non-primary profile (`--profile` still wins after merge).

| Surface | Runtime value | Tools (summary) | Read-only |
|---------|---------------|-----------------|-----------|
| Build | `build` | direct read/write/test tools in the main session | No |
| Plan | `plan` | read, glob, grep, web_search, todos | Yes (no writes; use **`/plan run`** or `/apply-plan` to execute) |
| Builder | `builder` | broader direct-coding surface | No |
| Explore | `explore` | read, glob, grep, web, todos | Yes |
| Verification | `verification` | read_file, bash, script, todos | No (checks only) |
| Code review | `code-review` | read, grep, bash, web, todos (no writes) | No (writes not on allowlist) |
| Guide | `guide` | none | Yes |
| StatusLine | `statusline` | none | Yes |
| Coordinator | `coordinator` | spawn_agent, stop_task, todo_write | Yes |

Coordinator delegates work to workers; see [coordinator.md](./coordinator.md).

### Coordinator vs direct coding (`build`)

Use **`coordinator`** when you want the hub to delegate sub-tasks to isolated workers via `spawn_agent`. Use **`build`** when you want a single agent to edit the repository directly without that extra layer — fewer LLM rounds and usually faster for straightforward tasks.

### `spawn_agent`: time and visibility

- Each **one-shot** `spawn_agent` runs a full worker loop (LLM + tools) until it finishes or hits **`timeout_sec`** (default **120**, maximum **600** seconds). The footer shows elapsed time while the tool runs.
- Worker assistant output is **streamed to the same transcript** as the parent session when using the interactive TUI, so you can see tokens as the worker produces them (not only after the tool completes).
- **`interactive: true`** returns immediately with a `task_id` and a `running` status; use **`/focus`** in the REPL to send more messages to that worker. The **first** worker turn is also streamed when the UI provides a sink.

### Parallel tool runs and duplicate `spawn_agent`

If the model requests **multiple tools** in one assistant message and they are auto-approved (allow mode or YOLO), goclaw may run those tools **in parallel**. **`spawn_agent` is never parallelized with other tools in the same batch** — it always runs sequentially to reduce duplicated work and resource contention (for example, two workers competing for the same local GPU).

If you still see two completed spawn lines for the same task, the model may have issued **two `spawn_agent` calls across iterations**; narrow the request or switch to **`build`** to avoid unnecessary delegation.

### Interactive workers (`spawn_agent` + REPL focus)

When the coordinator calls `spawn_agent` with **`"interactive": true`**, the tool returns immediately with `"status": "running"` and a `task_id`. The worker keeps running in the background. In the REPL:

- **`/workers`** — list interactive workers (id, profile, status, summary).
- **`/focus <task_id_prefix>`** — route typed messages to that worker until **`/detach`** (or `/focus parent`).
- **`stop_task`** — same as before; cancels the worker by `task_id`.

In the TUI, tool approval for **ask** mode appears as a **single compact line above the input** with **`y` / `n`** handling.

## Built-in tools (summary)

| Tool | Role |
|------|------|
| `read_file`, `glob`, `grep` | Read/search workspace |
| `bash` | One simple command; allowlist; timeout (default 30s, override `bash_timeout_sec`) |
| `script` | Multi-line shell (**on by default**; opt-out `allow_script: false`); same timeout as `bash` |
| `write_file`, `edit_file`, `patch` | Writes (stripped on read-only profiles) |
| `web_fetch`, `web_search` | Network (SSRF rules on fetch) |
| `todo_write` | Session task list |
| `spawn_agent`, `stop_task` | Coordinator only — start / cancel isolated workers |

Caps, SSRF, and MCP naming (`mcp__<id>__<name>`): [tool-contract.md](../reference/tool-contract.md). Visual tool flows (diagrams): [tool-flows.md](../reference/tool-flows.md). Workspace path rules: `internal/tools/workspace_paths.go`.

**Web:** use `web_search` for discovery; `web_fetch` when you already have a URL.

## Permissions

- `ask` (default) — prompt in the TUI (compact approval strip) before running  
- `allow` — no prompt  
- `deny` — block  

Configure in `tool_permissions`. Unlisted MCP tools default to `ask`.

## CLI (flags and subcommands)

Persistent flags apply to the default command and `chat`:

| Flag | Purpose |
|------|---------|
| `--profile` | Agent profile |
| `--session` | Resume session id |
| `--list-sessions` | Print ids and exit |
| `--no-tools` | Chat-only |
| `--tui` | Force fullscreen TUI (default on TTY when stdin/stdout are interactive) |
| `--mock` | Canned reply (no model) |
| `--output-format` | `text` or `json` for one-shot stdout |
| `--json-output` | Stdin automation → JSON |
| `--plugin-dir` | Plugin roots (repeatable); merges with `plugin_dirs` in settings |
| `--task-model-router` | `off` \| `rules` \| `llm` — per-turn **`task_models`** routing; see [model-routing.md](./model-routing.md) |

| Subcommand | Purpose |
|------------|---------|
| `chat` | Interactive session (same as default) |
| `prompt <text>...` | One turn from argv |
| `doctor` | Preflight check — Bubble Tea pager on TTY, plain print otherwise (`task_model_router` / `task_models` when set) |
| `sessions list` | Same as `--list-sessions` |

## Optional integrations

These are supported, but intentionally outside the default local-first path:

- **Telegram bridge:** `goclaw telegram start`, `goclaw telegram configure`, `goclaw telegram bridge`, `goclaw telegram user add ...` — see [telegram-bridge.md](./telegram-bridge.md)
- **Model routing:** [model-routing.md](./model-routing.md)
- **Plugins and skills:** [plugins.md](../reference/plugins.md), [skills.md](../reference/skills.md)
- **Coordinator hub mode:** /profile coordinator and [coordinator.md](./coordinator.md)
- **Swarm notes:** [swarm.md](./swarm.md) is reference-only here

## Plan file (`.goclaw/plan.md`) and mini plans (`.goclaw/plans/`)

This mirrors the usual **plan → review → execute** loop (similar in spirit to [Cursor Plan Mode](https://cursor.com/docs/agent/planning)): keep intent in **Markdown on disk**, then run **one execution turn** per **`/apply-plan`** / **`/plan run`** — local models often need **several turns** for a large plan, so split work into **small plans** with verifiable **## Steps**.

Use profile **`plan`** to draft text without touching the repo; then save and execute from the REPL.

The **`plan`** profile prioritizes **written analysis and a structured markdown plan** in the transcript. Optional read-only tools (`read_file`, `glob`, `grep`, `web_search`) are for **grounding** claims in the repo or in external facts — they do not replace reasoning. **Execution** (edits, verify loops, applying steps to disk) happens after you save and run **`/plan run`** or **`/apply-plan`**, which switches to **`build`** or **`coordinator`** — not while you stay on the plan profile.

- `/plan init` — create `.goclaw/plan.md` from template (if missing)
- **`/plan new my-feature`** — create **`.goclaw/plans/my-feature.md`** from a compact template (name is sanitized to a safe filename stem). Edit the file in your editor, then **`/apply-plan .goclaw/plans/my-feature.md`** (or **`/plan run .goclaw/plans/my-feature.md`** after saving the latest assistant line into that file).
- `/plan save` — save the last assistant message to **`.goclaw/plan.md`** (default). **`/plan save .goclaw/plans/foo.md`** saves to another path under the workspace (same rule as **`/apply-plan`** paths).
- **`/plan run`** (alias **`/plan apply`**) — save the last assistant message, then run the same path as **`/apply-plan`** (one model turn). Optional **`--hub`**. Optional plan path: **`/plan run --hub .goclaw/plans/foo.md`** saves into that file then executes it. When **`plan_apply_use_coordinator`** is true in settings, execution uses the coordinator profile even without **`--hub`**.
- `/plan path` / `/plan template` — inspect the default path and template skeleton
- **`/plan review`** — print a bounded excerpt, approval status, and **parsed `## Steps`** lines (optional plan path argument).
- **`/plan approve`** — write **`.goclaw/plan.meta.json`** with a SHA-256 of the current plan file so **`/apply-plan`** / **`/plan run`** can proceed when **`plan_require_apply_approval`** is true. Re-run after any edit to the plan file.
- **`/plan revoke`** — delete **`plan.meta.json`** (clear approval).
- **`/plan steps`** — list parsed steps only (expects a **`## Steps`** section with numbered **`1.`** or **`-` / `*`** lines; used to steer the handoff message).
- `/apply-plan [--preview] [--hub] [path]` — **`--preview`**: print a bounded excerpt (no model call, no profile switch). **Without `--preview`**: load the plan, switch to **`build`** (or **`coordinator`** with **`--hub`** or **`plan_apply_use_coordinator`**), and stream **one** execution turn.

**Structured steps:** add a markdown heading **`## Steps`** and numbered lines (`1. …`, `2. …`) so goclaw can inject a short ordered checklist into the handoff. Default execution policy is **sequential** (especially under coordinator); parallel **`spawn_agent`** is left to the model only when steps are clearly independent.

Typical workflow: `/mode plan` → ask for a plan → **`/plan review`** → **`/plan approve`** (if your project sets **`plan_require_apply_approval`**) → **`/plan run`** **or** `/plan save` → `/apply-plan --preview` (optional) → **`/apply-plan`** (add **`--hub`** for multi-worker hub mode). For coding that stalls in prose, switch to **`/mode build`** or **`/profile builder`**, use **mini plans** under **`.goclaw/plans/`**, and send **follow-up** turns until **`## Steps`** are done. In the **TUI**, **Ctrl+P** opens the agent profile picker. Hide built-in clutter with **`agent_picker_hidden_profiles`**. See [agent-profiles.md](../reference/agent-profiles.md).

## Slash commands (REPL)

Handled locally (not sent to the model). Run **`/help`** for the full list. Key groups:

- **Session:** `/new`, `/save`, `/session`, `/sessions`, `/resume`, `/compact`
- **Navigation:** `/focus <id>`, `/detach` (aliases: `/back`, `/hub`, `/parent`, `/in`), `/workers`
- **Content:** `/copy`, `/export`, `/memory`, `/plan`, `/apply-plan`, `/audit`, `/review` (see [code-review-workflow.md](./code-review-workflow.md))
- **Config:** `/profile`, `/agents`, `/theme`, `/init`, `/doctor`
- **UI:** `/clear` (same idea as Ctrl+L), `/edit` (multiline via $EDITOR), `/capabilities`, `/help`

**`/btw`** consumes the line but **submits** a rewritten user message to the model. **Prefix** lines `!`, `@`, `&` run tools locally then record user + assistant text in the session (see [prefix-input-modes.md](./prefix-input-modes.md)). Same health output as `goclaw doctor` when `/doctor` is wired in the REPL.

## Hooks, MCP, IDE ping

- **Hooks:** `PreToolUse`, `PostToolUse`, session start/end; Go API, `external_hooks` in settings, optional `.goclaw/hooks.json` when `trusted_workspace`. [hooks.md](../reference/hooks.md)
- **MCP:** `mcp_servers` in settings; stdio and streamable HTTP; optional `bearer_token_file` on URL servers. [mcp.md](../reference/mcp.md), [mcp-remote.md](./mcp-remote.md)
- **Plugins / skills:** [CLAUDE.md](../../goclaw/CLAUDE.md) D20 and "Skills (runtime)" row
- **IDE:** `GOCLAW_IDE_NOTIFY_URL` — localhost POST after each tool (best-effort). [ide-bridge.md](../reference/ide-bridge.md)

## Development

```bash
go vet ./...
go test ./...
go test -race ./...    # Linux CI; Windows needs CGO toolchain
make parity            # mock OpenAI-compat harness (orchestrator + coordinator)
```

Mock server: `testutil/mockopenai/`. Windows: transient `*.exe` from tests are normal; see `.gitignore`.

## Troubleshooting

- **Ollama connection refused** — start `ollama serve` or set `OLLAMA_HOST`.
- **Thin `web_search` results** — narrow the query or `web_fetch` a known URL.
- **Non-interactive tools fail** — set `tool_permissions` to `allow` for those tools or use `--no-tools`.

### Assistant explains plans but does not modify files

Work through this list in order (most issues are **profile**, **permissions**, or **model tool-calling**):

1. **Active profile** — In the REPL, note the profile name (footer / welcome) or run **`/mode build|plan`** / **`/profile <name>`** with a name from **`/agents`**. **`plan`**, **`explore`**, **`guide`**, and **`statusline`** cannot run `write_file` / `edit_file` / **`patch`**. **`coordinator`** does not touch the repo on the parent session — it delegates via **`spawn_agent`**. **`code-review`** is review-only (no write tools). For direct edits on the main session, Use **`build`** or **`builder`**.
2. **Plan workflow** — If you used **`plan`** to draft steps, that profile is **intentionally** read-only. Use **`/plan review`** / **`/plan approve`** (when **`plan_require_apply_approval`** is true), then **`/plan run`** or **`/plan save`** + **`/apply-plan`**. Add **`--hub`** (or set **`plan_apply_use_coordinator`**) for **coordinator** execution. Each apply runs **one** model turn; large plans may need follow-up messages or a smaller plan slice.
3. **Tools disabled** — **`--no-tools`**, **`GOCLAW_DISABLE_TOOLS=1`**, or **`goclaw prompt … --no-tools`** registers no tools: the model can only emit text. Remove the flag or unset the variable for edits.
4. **`tool_permissions`** — Default **`ask`** prompts before risky tools. If you decline or never approve, no run occurs. For scripts / CI, set **`allow`** on the tools you need (see [Permissions](#permissions)) or use **`/allow-writes`** in the REPL for the current session (write tools only).
5. **Ollama model** — Small local models often emit **prose or fake “JSON tool” blobs** instead of native tool calls; those blobs do **not** execute (see the base system prompt in `goclaw/internal/orchestrator/base_system_prompt.md`). Try a larger or more tool-capable tag (`ollama_model` / **`task_models`** entries) — [ollama-stack.md](./ollama-stack.md).
6. **Custom agents** — Markdown agents under **`~/.goclaw/agents/`** and **`.goclaw/agents/`** may set **`read_only: true`** or a narrow **`tool_allowlist`** without write tools; same symptom as built-in read-only profiles.
7. **Auto-continue nudges** — Goclaw may inject up to **`auto_continue_action_max_nudges`** synthetic `[goclaw]` user lines (default **2**, max **5**) when your message signals code changes and the model replies **without** native tool calls — including the **first** completion in the turn (zero tools) or after **read-only** tools only. If the model still ignores tools after nudges, use a stronger **Ollama** tag or **`/profile builder`**.
8. **Truth-on-disk footer** — If you see **`[goclaw] No workspace files were modified this turn`**, the runtime is reporting that no **`write_file` / `edit_file` / `patch`** succeeded; treat it as ground truth over assistant prose. See the **Truth-on-disk footer** bullet under [Large repo analysis and refactors](#large-repo-analysis-and-refactors).

## Documentation map

| Need | Location |
|------|----------|
| What lives where (module vs `docs/`) | [documentation.md](documentation.md) |
| Master index (all `.md` paths) | [docs-map.md](../docs-map.md) |
| Tool limits, SSRF, MCP naming | [tool-contract.md](../reference/tool-contract.md) |
| Visual tool flows (diagrams) | [tool-flows.md](../reference/tool-flows.md) |
| English architecture blurb + diagram | [architecture.md](../architecture.md) |

