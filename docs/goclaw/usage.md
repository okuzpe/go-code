# goclaw — Usage

Task-oriented guide: how to run the CLI, configure providers, and use sessions, tools, and profiles. Deep reference for tools, limits, and architecture lives in [CLAUDE.md](../../goclaw/CLAUDE.md). Documentation: [README.md](../../goclaw/README.md), [docs-map.md](../docs-map.md).

## Quick start (Ollama)

Prerequisite: Ollama on `http://localhost:11434` with a model pulled (default: `qwen2.5-coder:14b`; use `qwen2.5-coder:7b` in settings on tight VRAM).

```bash
cd goclaw
go run ./cmd/goclaw doctor
go run ./cmd/goclaw              # fullscreen TUI on TTY (default)
go run ./cmd/goclaw --readline   # line REPL; or GOCLAW_USE_TUI=0
```

Try: a simple repo question, a tool (e.g. web search), and `/doctor` or `goclaw doctor`.

### Large repo analysis and refactors

- **`.goclaw/plan.md`** is a local scratchpad; it may contain notes unrelated to goclaw. For product intent and architecture, prefer **[CLAUDE.md](../../goclaw/CLAUDE.md)**, **[README.md](../../goclaw/README.md)**, and the topic files under **[docs/goclaw/](./)** (see [docs-map.md](../docs-map.md)).
- For **“audit everything”** or whole-tree refactors: ask for **one slice per turn** (one package, one `internal/` area, or one doc). Check results with **`git diff`**, tool output, or successful write tools — not with narrative alone.
- **Disk changes** only happen when **`write_file`**, **`edit_file`**, or **`patch`** complete successfully (subject to permissions). If the assistant says it “applied changes” but no write tool ran, treat that as prose, not git truth.
- **Why it stopped at "Done"** — Each of your messages runs until the model answers **without** requesting more tools. A text-only reply (even "Done") **ends that turn**; there is no hidden auto-loop. For multi-step refactors, either write one prompt that forces **read → edit → verify** in one go, or send a **follow-up** ("continue: apply the first edit to `internal/...`").
- **Auto-continue (default on)** — When your message clearly asks for fixes/refactors and the model answers with **prose only** — including the **first** completion with **zero** tool calls (common with small local models) **or** after **read-only** tools only — goclaw may inject a short `[goclaw]` user line and **re-prompt the model** (default **2** times per user message for both patterns combined). Optional **`auto_continue_action_max_nudges`** in `settings.json` raises that cap (**1–5**; values above **5** are clamped). Set `"auto_continue_action_requests": false` to disable entirely. If nudges exhaust and tools still never run, see [Troubleshooting — Assistant explains plans but does not modify files](#assistant-explains-plans-but-does-not-modify-files).
- **Truth-on-disk footer (default on)** — If your message signals code changes, tools ran, writes are allowed for the profile, but no `write_file` / `edit_file` / `patch` completed successfully, the runtime may append a short bilingual `[goclaw]` footer to the assistant reply (and session log) so prose cannot claim edits alone. Set `"truth_footer_no_workspace_writes": false` in `settings.json` to disable. **After successful workspace writes:** both **TUI** and **`--readline`** may show a **`goclaw: git diff --stat`** block when the workspace is a git repo (bounded runtime and output size). **TUI vs readline for the rest:** after the truth footer, the TUI shows an extra hint under the session footer. **`--readline`** also prints a **`goclaw:` stderr** recap each successful turn (`turn complete (no tools)` or `N tool call(s): …`); when the reply includes the truth footer, stderr suggests **continue**, **`/continue`**, or a short follow-up. Recap lines are **not** printed if the turn errors or is cancelled mid-flight.
- **Turn shape** — One user message runs the loop until the model returns **no** tool calls (then the turn ends). If it stopped after reads only, typing **continue**, slash **`/continue`**, or nudging the agent is normal; **`/continue`** sends a standard follow-up while keeping full session history (see `/help`).
- **Context window** — If **`ollama_num_ctx`** is set in settings and is **below 8192** (see `OllamaNumCtxBannerWarnBelow` in `internal/app/ux_constants.go`), the **readline / non-TTY startup banner** prints a short warning: long system prompts plus tool schemas may truncate. Raise `ollama_num_ctx` if turns feel “forgetful” or tools behave oddly.
- **Iteration budget** — Past halfway through the per-turn iteration limit, a `<system-reminder>` warns the model that budget is tight. If you asked for real edits and tools already ran without a successful write, that reminder nudges toward **finishing edits** instead of only wrapping up in prose.

### First-run setup (onboarding)

The first time you run **interactive** goclaw on a TTY and **`~/.goclaw/settings.json` does not exist**, a short wizard runs **before** the chat UI:

1. Security summary (optional full text is bundled; same content as [security.md](./security.md))
2. Workspace trust for the current directory (`trusted_workspace` in project `.goclaw/settings.json`)
3. **TUI appearance** preset (fullscreen mode only; change later with `/theme`)
4. **Provider**: Ollama (local); settings are written under `~/.goclaw/` as described below

**Files written:** `~/.goclaw/settings.json` (and `settings.local.json` if you enter an API key); project `.goclaw/settings.json` when you confirm trust.

**Environment:**

- `GOCLAW_NO_ONBOARDING=1` — skip the wizard (advanced; you still need safe usage practices — see [security.md](./security.md))
- `GOCLAW_ONBOARDING=1` — force the wizard even if `settings.json` already exists (useful for testing)

**`goclaw doctor` does not run onboarding** — it loads config and prints a health report. Run `doctor` for a quick check; run `goclaw` once to complete first-time setup.

The wizard follows the **same TUI vs readline** rules as the main app (default fullscreen TUI on a TTY unless `GOCLAW_USE_TUI=0` or `--readline`). The default **agent profile** is **`general-purpose`** until you set `agent_profile` or use `/profile` — see [Agent profiles](#agent-profiles).

### REPL modes

- **TUI (default on a TTY)** — fullscreen Bubble Tea: transcript, compact tool approval above the input, `/focus` hint in the footer. Opt out with `GOCLAW_USE_TUI=0` or `--readline` / `GOCLAW_USE_READLINE=1`. The ASCII startup banner is **not** printed to stdout in this mode (welcome panel + footer carry session context).
- **Readline** — line-at-a-time claw-style prompt; `make run-readline` or `goclaw --readline`. Prints the startup banner (TTY: styled; non-TTY: plain lines with workspace and session).

Exit: `Esc` (TUI) or `Ctrl+C`. Clear: `Ctrl+L` (TUI).

### Slash commands, autocomplete, and help

- **TUI (fullscreen)** — Type `/` on a **single line** to see a **filtered list** of commands as you keep typing (prefix match). **Tab** completes the command (longest shared prefix, or the only match). After the command name, the strip shows **argument** suggestions where supported (e.g. `/profile`, `/memory`, `/plan`, `/resume`, `/focus`, `/theme`, `/export`); **Tab** completes the argument token at the cursor the same way. The command list matches the readline completer (one source of truth).
- **`/help` in the TUI** — Opens a **dismissible help panel** over the transcript (same text as the slash handler). **Esc** closes the panel; **↑** / **↓** (or `k` / `j`) and **PgUp** / **PgDn** scroll long output. **Ctrl+C** still quits the app from the panel.
- **Readline** — **Tab** completes `/` commands via the readline prefix completer, and **slash arguments** after the first token when the REPL has a live session context (same rules as the TUI). **`/help`** prints the full help text **inline** in the transcript (no overlay).

### Prefix input (`!`, `@`, `&`, `/btw`)

Interpreted **after** slash commands and **before** the model (TUI and readline). Same **permission policy, approval, and hooks** as normal tool calls. **Single line** for `!` and `&` (extra lines are rejected). **`--mock`** disables prefix handling.

| Prefix | Meaning |
|--------|---------|
| `!` + command | Run the **`bash`** tool with that command (allowlist and metacharacter rules apply). |
| `@` + path (standalone) | Run **`read_file`** for a path inside the workspace. **TUI:** matching paths appear under the input as you type; **Tab** completes anywhere in the line. **Readline:** **Tab** completes `@` paths or `/` commands. Drag-and-drop a file/folder onto the terminal to insert `@relpath` automatically. |
| `@token` inline | When `@path` tokens appear inside a larger message (e.g. `explain @go.mod`), the file is silently pre-loaded before the model call — no separate read step needed. |
| `&` + task | Run **`spawn_agent`** with profile `general-purpose` (requires **`spawn_agent`** on the active profile — use **`/profile coordinator`** or hub mode for that). |
| `/btw` + text | Slash command: submit **one** user message wrapped as a short “side question” to the model. |

Full grammar and security notes: [prefix-input-modes.md](./prefix-input-modes.md).

## Sessions and memory

Sessions save as JSONL under `~/.goclaw/sessions/<id>.jsonl` on exit or `/save`.

**Copy / export chat (plain text from the session, not the styled TUI):**

- **`/copy`** — copies the in-memory transcript (roles, text, tool calls/results) to the **system clipboard**. Very long sessions are truncated (see `/help`). If the clipboard fails (headless SSH, etc.), use **`/export`**.
- **`/export path.txt`** — writes the same plain text to **`path.txt`**. If the path is relative and a workspace is set, it is resolved under that workspace; use an absolute path to write elsewhere.

The fullscreen TUI keeps **normal terminal mouse behaviour** by default (wheel scrolling is **off** so click–drag selection works as your host terminal allows). Enable wheel-on-transcript with **`tui_mouse_scroll`**: `true` in settings or **`GOCLAW_TUI_MOUSE_SCROLL=1`**. Use **Ctrl+B** in the TUI to scroll the transcript with the keyboard without capturing the mouse. For “everything in the session”, prefer `/copy` or `/export` over selecting the screen.

```bash
go run ./cmd/goclaw --list-sessions
go run ./cmd/goclaw sessions list
go run ./cmd/goclaw --session <id>
```

**Memory** (cross-session Markdown under `~/.goclaw/memory/`): types `user`, `feedback`, `project`, `reference`. Use `/memory list|add|delete` in the REPL.

## One-shot automation (`prompt` and JSON)

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

**Common keys:** `provider` (must be `ollama` for a normal run; legacy values error at startup), `agent_profile`, `ollama_model`, `ollama_host`, `ollama_num_ctx`, `bash_timeout_sec`, `tool_permissions`, `auto_continue_action_requests`, `auto_continue_action_max_nudges`, `truth_footer_no_workspace_writes`, `mcp_servers` (stdio or HTTP; HTTP entries may set `bearer_token_file` for a static bearer token), `mcp_allow_remote_urls`, `trusted_workspace`, `external_hooks`, `plugin_dirs`, `plugin_allow`, `plugin_deny`, `memory_auto_extract`, `ide_bridge_mcp`. CLI: `--plugin-dir` (repeatable) appends plugin roots.

Example:

```json
{
  "provider": "ollama",
  "agent_profile": "explore",
  "ollama_model": "qwen2.5-coder:14b",
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

**Default (no settings):** `general-purpose` — full tools on the main session. Use **`coordinator`** when you want hub mode (delegate with `spawn_agent` only on the parent).

Set with `--profile <name>`, `agent_profile` in settings, or **`GOCLAW_AGENT_PROFILE`** (applied after settings merge; `--profile` still wins).

| Profile | Value | Tools (summary) | Read-only |
|---------|-------|-----------------|-----------|
| General-Purpose | `general-purpose` | All built-ins + MCP | No |
| Explore | `explore` | read, glob, grep, web, todos | Yes |
| Plan | `plan` | read, glob, grep, web_search, todos | Yes (no writes; use **`/plan run`** or `/plan save` then `/apply-plan`) |
| Verification | `verification` | read_file, bash, script, todos | No (no write tools; checks only) |
| Code review | `code-review` | read, grep, bash, web, todos (no writes) | No (writes not on allowlist) |
| Guide | `guide` | none | Yes |
| StatusLine | `statusline` | none | Yes |
| Coordinator | `coordinator` | spawn_agent, stop_task, todo_write | Yes |

Coordinator delegates work to workers; see [coordinator.md](./coordinator.md).

### Coordinator vs direct coding (`general-purpose`)

Use **`coordinator`** when you want the hub to delegate sub-tasks to isolated workers via `spawn_agent`. Use **`general-purpose`** when you want a single agent to edit the repository directly without that extra layer — fewer LLM rounds and usually faster for straightforward tasks (for example, a small desktop app or a single feature).

### `spawn_agent`: time and visibility

- Each **one-shot** `spawn_agent` runs a full worker loop (LLM + tools) until it finishes or hits **`timeout_sec`** (default **120**, maximum **600** seconds). The footer shows elapsed time while the tool runs.
- Worker assistant output is **streamed to the same transcript** as the parent session when using the interactive TUI or readline REPL, so you can see tokens as the worker produces them (not only after the tool completes).
- **`interactive: true`** returns immediately with a `task_id` and a `running` status; use **`/focus`** in the REPL to send more messages to that worker. The **first** worker turn is also streamed when the UI provides a sink.

### Parallel tool runs and duplicate `spawn_agent`

If the model requests **multiple tools** in one assistant message and they are auto-approved (allow mode or YOLO), goclaw may run those tools **in parallel**. **`spawn_agent` is never parallelized with other tools in the same batch** — it always runs sequentially to reduce duplicated work and resource contention (for example, two workers competing for the same local GPU).

If you still see two completed spawn lines for the same task, the model may have issued **two `spawn_agent` calls across iterations**; narrow the request or switch to **`general-purpose`** to avoid unnecessary delegation.

### Interactive workers (`spawn_agent` + REPL focus)

When the coordinator calls `spawn_agent` with **`"interactive": true`**, the tool returns immediately with `"status": "running"` and a `task_id`. The worker keeps running in the background. In the REPL:

- **`/workers`** — list interactive workers (id, profile, status, summary).
- **`/focus <task_id_prefix>`** — route typed messages to that worker until **`/detach`** (or `/focus parent`).
- **`stop_task`** — same as before; cancels the worker by `task_id`.

In the TUI, tool approval for **ask** mode appears as a **single compact line above the input**; readline prints one approval line on stderr before the `Allow execution?` prompt.

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

- `ask` (default) — prompt on stderr before running  
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
| `--tui` / `--readline` | UI mode |
| `--mock` | Canned reply (no model) |
| `--output-format` | `text` or `json` for one-shot stdout |
| `--json-output` | Stdin automation → JSON |
| `--plugin-dir` | Plugin roots (repeatable); merges with `plugin_dirs` in settings |
| `--task-model-router` | `off` \| `rules` \| `llm` — per-turn **`task_models`** routing; see [model-routing.md](./model-routing.md) |

| Subcommand | Purpose |
|------------|---------|
| `chat` | Interactive session (same as default) |
| `prompt <text>...` | One turn from argv |
| `doctor` | Preflight check (shows `task_model_router` / `task_models` when set) |
| `sessions list` | Same as `--list-sessions` |

## Plan file (`.goclaw/plan.md`)

Use profile `plan` to draft the plan as chat output. In the REPL:

- `/plan init` — create `.goclaw/plan.md` from template
- `/plan save` — save the last assistant message in this session to `.goclaw/plan.md`
- **`/plan run`** (alias **`/plan apply`**) — save the last assistant message, then immediately run the same execution path as **`/apply-plan`** (switch to **`general-purpose`**, one model turn). Optional review first: **`/apply-plan --preview`**, then **`/plan run`** or **`/apply-plan`**.
- `/plan path` / `/plan template` — inspect the default path and template skeleton
- `/apply-plan [--preview] [path]` — **`--preview`**: print a bounded excerpt of the plan on disk (no model call, no profile switch). **Without `--preview`**: load the plan, switch to `general-purpose`, and stream **one** execution turn (same as before).

Typical workflow: `/profile plan` → ask for a plan → **`/plan run`** **or** `/plan save` → `/apply-plan --preview` (optional) → `/apply-plan`. In the **TUI**, **Ctrl+P** opens the agent profile picker (same as typing `/agents` and Enter). See [agent-profiles.md](../reference/agent-profiles.md).

## Slash commands (REPL)

Handled locally (not sent to the model). Run **`/help`** for the full list. Key groups:

- **Session:** `/new`, `/save`, `/session`, `/sessions`, `/resume`, `/compact`
- **Navigation:** `/focus <id>`, `/detach` (aliases: `/back`, `/hub`, `/parent`, `/in`), `/workers`
- **Content:** `/copy`, `/export`, `/memory`, `/plan`, `/apply-plan`, `/audit`, `/review` (see [code-review-workflow.md](./code-review-workflow.md))
- **Config:** `/profile`, `/agents`, `/theme`, `/init`, `/doctor`
- **UI:** `/clear` (same as Ctrl+L in readline), `/edit` (multiline via $EDITOR), `/capabilities`, `/help`

**`/btw`** consumes the line but **submits** a rewritten user message to the model. **Prefix** lines `!`, `@`, `&` run tools locally then record user + assistant text in the session (see [prefix-input-modes.md](./prefix-input-modes.md)). Same health output as `goclaw doctor` when `/doctor` is wired in the REPL.

## Hooks, MCP, IDE ping

- **Hooks:** `PreToolUse`, `PostToolUse`, session start/end; Go API, `external_hooks` in settings, optional `.goclaw/hooks.json` when `trusted_workspace`. [hooks.md](../reference/hooks.md)
- **MCP:** `mcp_servers` in settings; stdio and streamable HTTP; optional `bearer_token_file` on URL servers. [mcp.md](../reference/mcp.md), [mcp-remote.md](./mcp-remote.md)
- **Plugins / skills / swarm:** [CLAUDE.md](../../goclaw/CLAUDE.md) D20 and “Skills (runtime)” row; [swarm.md](./swarm.md)
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

1. **Active profile** — In the REPL, note the profile name (footer / welcome) or run **`/profile <name>`** with a name from **`/agents`**. **`plan`**, **`explore`**, **`guide`**, and **`statusline`** cannot run `write_file` / `edit_file` / **`patch`**. **`coordinator`** does not touch the repo on the parent session — it delegates via **`spawn_agent`**. **`code-review`** is review-only (no write tools). For direct edits on the main session, use **`general-purpose`** or **`builder`** (`agent_profile` in `settings.json`, **`GOCLAW_AGENT_PROFILE`**, or **`goclaw --profile …`** — merge order in [Configuration](#configuration)).
2. **Plan workflow** — If you used **`plan`** to draft steps, that profile is **intentionally** read-only. Use **`/plan run`** to save the latest assistant message and start execution in one step, **or** **`/plan save`** then **`/apply-plan`** (optional **`/apply-plan --preview`** first). Execution switches to **`general-purpose`** and runs **one** model turn; large plans may need a **follow-up user message** or a smaller plan slice.
3. **Tools disabled** — **`--no-tools`**, **`GOCLAW_DISABLE_TOOLS=1`**, or **`goclaw prompt … --no-tools`** registers no tools: the model can only emit text. Remove the flag or unset the variable for edits.
4. **`tool_permissions`** — Default **`ask`** prompts before risky tools. If you decline or never approve, no run occurs. For scripts / CI, set **`allow`** on the tools you need (see [Permissions](#permissions)) or use **`/allow-writes`** in the REPL for the current session (write tools only).
5. **Ollama model** — Small local models often emit **prose or fake “JSON tool” blobs** instead of native tool calls; those blobs do **not** execute (see the base system prompt in `goclaw/internal/orchestrator/base_system_prompt.md`). Try a stronger coder tag (e.g. **`qwen2.5-coder:14b`** vs 7b) — [ollama-stack.md](./ollama-stack.md).
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
