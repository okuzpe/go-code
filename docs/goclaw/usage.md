# goclaw — Usage

Task-oriented guide: how to run the CLI, configure providers, and use sessions, tools, and profiles. Deep reference for tools, limits, and architecture lives in [CLAUDE.md](../../goclaw/CLAUDE.md). Documentation: [README.md](../../goclaw/README.md), [docs-map.md](../docs-map.md).

## Quick start (Ollama)

Prerequisite: Ollama on `http://localhost:11434` with a model pulled (default: `qwen2.5-coder:14b`).

```bash
cd goclaw
go run ./cmd/goclaw doctor
go run ./cmd/goclaw              # fullscreen TUI on TTY (default)
go run ./cmd/goclaw --readline   # line REPL; or GOCLAW_USE_TUI=0
```

Try: a simple repo question, a tool (e.g. web search), and `/doctor` or `goclaw doctor`.

### First-run setup (onboarding)

The first time you run **interactive** goclaw on a TTY and **`~/.goclaw/settings.json` does not exist**, a short wizard runs **before** the chat UI:

1. Security summary (optional full text is bundled; same content as [security.md](./security.md))
2. Workspace trust for the current directory (`trusted_workspace` in project `.goclaw/settings.json`)
3. **TUI appearance** preset (fullscreen mode only; change later with `/theme`)
4. **Provider**: Ollama or Anthropic (API key is written to `~/.goclaw/settings.local.json`)

**Files written:** `~/.goclaw/settings.json` (and `settings.local.json` if you enter an API key); project `.goclaw/settings.json` when you confirm trust.

**Environment:**

- `GOCLAW_NO_ONBOARDING=1` — skip the wizard (advanced; you still need safe usage practices — see [security.md](./security.md))
- `GOCLAW_ONBOARDING=1` — force the wizard even if `settings.json` already exists (useful for testing)

**`goclaw doctor` does not run onboarding** — it loads config and prints a health report. Run `doctor` for a quick check; run `goclaw` once to complete first-time setup.

The wizard follows the **same TUI vs readline** rules as the main app (default fullscreen TUI on a TTY unless `GOCLAW_USE_TUI=0` or `--readline`). The default **agent profile** remains **coordinator** until you set `agent_profile` or use `/profile` — see [Agent profiles](#agent-profiles).

### REPL modes

- **TUI (default on a TTY)** — fullscreen Bubble Tea: transcript, compact tool approval above the input, `/focus` hint in the footer. Opt out with `GOCLAW_USE_TUI=0` or `--readline` / `GOCLAW_USE_READLINE=1`. The ASCII startup banner is **not** printed to stdout in this mode (welcome panel + footer carry session context).
- **Readline** — line-at-a-time claw-style prompt; `make run-readline` or `goclaw --readline`. Prints the startup banner (TTY: styled; non-TTY: plain lines with workspace and session).

Exit: `Esc` (TUI) or `Ctrl+C`. Clear: `Ctrl+L` (TUI).

### Slash commands, autocomplete, and help

- **TUI (fullscreen)** — Type `/` on a **single line** to see a **filtered list** of commands as you keep typing (prefix match). **Tab** completes the command (longest shared prefix, or the only match). The same list is defined in code as the readline completer (one source of truth).
- **`/help` in the TUI** — Opens a **dismissible help panel** over the transcript (same text as the slash handler). **Esc** closes the panel; **↑** / **↓** (or `k` / `j`) and **PgUp** / **PgDn** scroll long output. **Ctrl+C** still quits the app from the panel.
- **Readline** — **Tab** completes `/` commands via the readline prefix completer. **`/help`** prints the full help text **inline** in the transcript (no overlay).

## Sessions and memory

Sessions save as JSONL under `~/.goclaw/sessions/<id>.jsonl` on exit or `/save`.

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

## Anthropic (optional)

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

Set `"provider": "anthropic"` in `~/.goclaw/settings.json` or `.goclaw/settings.json` in the project.

| `GOCLAW_MODEL` | Resolves to (anthropic only) |
|----------------|------------------------------|
| *(default)* | `claude-sonnet-4-6` |
| `opus` | `claude-opus-4-6` |
| `sonnet` | `claude-sonnet-4-6` |
| `haiku` | `claude-haiku-4-5-20251213` |
| any other string | sent as-is to the API |

Ollama mode uses `ollama_model` / `OLLAMA_MODEL`; it does **not** use these aliases.

## Configuration

Merge order (later overrides earlier):

```
defaults → ~/.goclaw/settings.json → .goclaw/settings.json
        → ~/.goclaw/settings.local.json → .goclaw/settings.local.json
```

Do not commit `settings.local.json`.

**Common keys:** `provider`, `agent_profile`, `ollama_model`, `bash_timeout_sec`, `tool_permissions`, `mcp_servers` (stdio or HTTP; HTTP entries may set `bearer_token_file` for a static bearer token), `mcp_allow_remote_urls`, `trusted_workspace`, `external_hooks`, `plugin_dirs`, `plugin_allow`, `plugin_deny`, `memory_auto_extract`, `ide_bridge_mcp`. CLI: `--plugin-dir` (repeatable) appends plugin roots.

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

## Agent profiles

**Default (no settings):** `coordinator` — hub mode; delegate with `spawn_agent` and use `/profile general-purpose` when you want a single agent that edits the repo directly.

Set with `--profile <name>`, `agent_profile` in settings, or **`GOCLAW_AGENT_PROFILE`** (applied after settings merge; `--profile` still wins).

| Profile | Value | Tools (summary) | Read-only |
|---------|-------|-----------------|-----------|
| General-Purpose | `general-purpose` | All built-ins + MCP | No |
| Explore | `explore` | read, glob, grep, web, todos | Yes |
| Plan | `plan` | read, glob, grep, web_search, todos | Yes |
| Verification | `verification` | read_file, bash, todos | No |
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
| `script` | Multi-line shell (opt-in `allow_script`); same timeout as `bash` |
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

| Subcommand | Purpose |
|------------|---------|
| `chat` | Interactive session (same as default) |
| `prompt <text>...` | One turn from argv |
| `doctor` | Preflight check |
| `sessions list` | Same as `--list-sessions` |

## Plan file (`.goclaw/plan.md`)

Use profile `plan` to draft, save under `.goclaw/plan.md`. In the REPL: `/plan path`, `/plan init`, `/plan template`, `/apply-plan [path]` (loads plan, switches to `general-purpose`, one orchestrator turn). See [agent-profiles.md](../reference/agent-profiles.md).

## Slash commands (REPL)

Handled locally (not sent to the model): `/help`, `/doctor`, `/session`, `/sessions`, `/quit`, `/exit`, `/new`, `/save`, `/compact`, `/profile`, `/plan`, `/apply-plan`, `/memory`. Same health output as `goclaw doctor` when `/doctor` is wired in the REPL.

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
make parity            # mock Anthropic harness
```

Mock server: `testutil/mockserver/`. Windows: transient `*.exe` from tests are normal; see `.gitignore`.

## Troubleshooting

- **Ollama connection refused** — start `ollama serve` or set `OLLAMA_HOST`.
- **Thin `web_search` results** — narrow the query or `web_fetch` a known URL.
- **Non-interactive tools fail** — set `tool_permissions` to `allow` for those tools or use `--no-tools`.

## Documentation map

| Need | Location |
|------|----------|
| What lives where (module vs `docs/`) | [documentation.md](documentation.md) |
| Master index (all `.md` paths) | [docs-map.md](../docs-map.md) |
| Tool limits, SSRF, MCP naming | [tool-contract.md](../reference/tool-contract.md) |
| Visual tool flows (diagrams) | [tool-flows.md](../reference/tool-flows.md) |
| English architecture blurb + diagram | [architecture.md](../architecture.md) |
