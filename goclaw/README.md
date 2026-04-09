# goclaw

Go CLI coding agent — local-first with Ollama (`qwen2.5-coder:14b` by default), optional Anthropic API (`claude-sonnet-4-6`). No cloud required.

## Documentation map

- [`USAGE.md`](USAGE.md) — quick start, sessions, tools, common workflows
- [`ROADMAP.md`](ROADMAP.md) — prioritized checklist toward a shippable daily driver
- [`PHILOSOPHY.md`](PHILOSOPHY.md) — product principles and UX intent
- [`CLAUDE.md`](CLAUDE.md) — architecture + tool contract + implementation rules

## Why goclaw

- **Runs 100% locally with Ollama** — no API key, no cost, no data leaving your machine.
- **Seven purpose-built agent profiles** (`explore`, `plan`, `coordinator`, and more) with tool allowlists baked in — swap context with a single flag.
- **Persistent sessions and cross-session memory** — conversation history saved as JSONL; four memory types (`user`, `feedback`, `project`, `reference`) survive restarts.
- **Hooks and per-tool permissions** — intercept any tool call before execution; configure `ask`/`allow`/`deny` per tool in `settings.json`.

## Quick Start

```bash
cd goclaw
go run ./cmd/goclaw --tui
```

First-run health check:

```bash
go run ./cmd/goclaw doctor
```

Build a standalone binary:

```bash
go build -o goclaw ./cmd/goclaw
./goclaw
```

On a TTY, the default is a **readline REPL** with a `>` prompt (banner, popular slash hints, then free text to the model — same flow as claw `run_repl`). Use **`--tui`** or **`GOCLAW_USE_TUI=1`** for the fullscreen UI. **`--readline`** forces readline if you need to override TUI.

With Anthropic API:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
# add "provider": "anthropic" to ~/.goclaw/settings.json
go run ./cmd/goclaw
```

## Features

### Tools

| Tool | What it does | Security notes |
|------|-------------|----------------|
| `read_file` | Read file contents with optional line offset and limit | Workspace-scoped; symlinks outside workspace rejected; 512 KiB / 200-line cap |
| `glob` | List files matching a pattern | Workspace-scoped; no `..` traversal; max 500 matches |
| `grep` | Regex search across files or a directory | Workspace-scoped; skips binary files; max 200 matches, 512 KiB per file |
| `bash` | Run shell commands | Expanded binary allowlist (D4); **one simple command** — no pipes, `;`, `&&`, redirects, or `$(...)`; quote URLs containing `&`; user confirmation in Ask mode; **30 s** timeout (override `bash_timeout_sec` in settings, 1–3600); 256 KiB output cap |
| `write_file` | Write or overwrite a file | Workspace-scoped; atomic write (temp + rename); parent dir must exist; 1 MiB content cap; stripped from read-only profiles |
| `edit_file` | Targeted string replacement in a file | `old_string` must match exactly once (unless `replace_all: true`); preserves file mode; stripped from read-only profiles |
| `web_fetch` | Fetch a URL as text | SSRF-protected: RFC1918, loopback, and metadata endpoints blocked; max 5 redirects re-validated; 1 MiB cap, 30 s timeout |
| `web_search` | Search via DuckDuckGo | No API key required; returns up to 8 results with 2 KiB snippets; 15 s timeout |
| `todo_write` | Update a session-scoped task list | In-memory until exit; merged into context for planning-style turns |
| `spawn_agent` | Launch an isolated worker agent for a sub-task | Coordinator profile only; workers run with their own session; profile must be `explore`, `plan`, `verification`, or `general-purpose`; default 120 s timeout (max 600 s); workers cannot spawn coordinators |

#### Path resolution in file tools (Issue #11)

All workspace file tools are **workspace-scoped** and defend against **symlink escapes**.
At a high level:

- **`read_file`, `grep`, `edit_file`**: resolve the *full target path*, then `EvalSymlinks`, then verify the resolved path is still under the workspace root via `filepath.Rel`.
- **`write_file`**: resolves and `EvalSymlinks` the **parent directory** (the file may not exist yet), verifies the parent is under the workspace root, then writes atomically (temp + rename).

Implementation lives in [`internal/tools/workspace_paths.go`](internal/tools/workspace_paths.go) and per-tool guards (e.g. [`internal/tools/write_file.go`](internal/tools/write_file.go)).

**MCP tools:** each remote tool is registered as `mcp__<server_id>__<remote_tool_name>` (see [`../MCP.md`](../MCP.md) and [`../TOOL_CONTRACT.md`](../TOOL_CONTRACT.md)). Unlisted tools default to **ask** mode; add explicit keys such as `mcp__myserver__fetch` for `allow` / `deny` overrides.

### Agent Profiles

Select a profile with `--profile <name>` (or `-profile`) or set `agent_profile` in `settings.json`.

| Profile | `--profile` value | Tools available | Read-only | Notes |
|---------|-----------------|-----------------|-----------|-------|
| General-Purpose | `general-purpose` | All built-ins + any MCP tools | No | Default; full tool access |
| Explore | `explore` | read_file, glob, grep, web_fetch, web_search, todo_write | Yes | No bash; MCP tools blocked at execution |
| Plan | `plan` | read_file, glob, grep, web_search, todo_write | Yes | Architecture and planning tasks |
| Verification | `verification` | read_file, bash, todo_write | No | Returns PASS or FAIL with a brief reason |
| Guide | `guide` | (none) | Yes | Chat-only Q&A; never runs commands |
| StatusLine | `statusline` | (none) | Yes | Outputs a single short status line |
| Coordinator | `coordinator` | spawn_agent, todo_write | Yes | Decomposes tasks; delegates to isolated workers; never uses file or shell tools directly |

### Session & Memory

Sessions are persisted as JSONL under `~/.goclaw/sessions/<id>.jsonl` on exit. Resume a session with `--session <id>`; list saved sessions with `goclaw sessions list`, `--list-sessions`, or `/sessions` in the REPL.

Memory entries persist across sessions in `~/.goclaw/memory/` as Markdown files with YAML frontmatter. Four types:

| Type | Purpose |
|------|---------|
| `user` | Personal preferences, role, and background |
| `feedback` | Corrections and confirmed approaches from past sessions |
| `project` | Repository-specific facts, decisions, and deadlines |
| `reference` | Pointers to external resources (docs, dashboards, issue trackers) |

Manage memory with `/memory list`, `/memory add <type> <name> <text>`, and `/memory delete <file.md>`.

### Hooks

goclaw fires five lifecycle events. You can register **Go handlers** in code, **`external_hooks`** in settings (subprocess or HTTP POST), and optional **project hooks** from `.goclaw/hooks.json` when `trusted_workspace` is true. See [`../HOOKS.md`](../HOOKS.md).

| Event | When it fires | Blocking? |
|-------|--------------|-----------|
| `PreToolUse` | Before a tool executes | Yes — returning an error (or external hook exit code **2**) cancels the tool call |
| `PostToolUse` | After a tool succeeds | No — errors logged with `slog.WarnContext` |
| `PostToolUseFailure` | After a tool returns an error | No |
| `SessionStart` | REPL startup, before first prompt | No |
| `SessionEnd` | REPL shutdown, before saving session | No |

```go
reg := hooks.New()
reg.On(hooks.PreToolUse, func(ctx context.Context, e hooks.Event) error {
    // e.ToolName, e.Input (JSON) — return error to block
    return nil
})
```

### MCP servers (stdio)

Configure subprocess MCP servers in `settings.json` under `mcp_servers`. Each entry needs `id`, `command`, and optional `args`, `env`, `cwd`, or `disabled`. Servers are merged by `id` across the usual settings merge order; later files override the same `id`.

If a server fails to start or register tools, goclaw logs a warning and continues without that server’s tools (other servers and built-ins still work).

### IDE notifications (localhost)

Set **`GOCLAW_IDE_NOTIFY_URL`** to `http://127.0.0.1:PORT/...` or `http://localhost:...` (or `https` to the same hosts). After each tool completes, goclaw POSTs a small JSON body (`tool`, `result_bytes`, `is_error`). Failures are ignored (best-effort). Remote URLs are rejected.

### Permissions

Three modes, configurable per tool:

- `ask` (default) — prompts on stderr before each tool call: `Allow execution? [y/N]:`
- `allow` — executes without prompting
- `deny` — always rejects

Set via `tool_permissions` in `settings.json` (see Configuration below).

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama server URL |
| `OLLAMA_MODEL` | `qwen2.5-coder:14b` | Model name when `provider=ollama` |
| `ANTHROPIC_API_KEY` | — | Required when `provider=anthropic` |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Override for testing (e.g. mock server) |
| `GOCLAW_MODEL` | `claude-sonnet-4-6` | Anthropic model name |
| `GOCLAW_DISABLE_TOOLS` | — | Set to `1` for chat-only mode (no tools registered) |
| `GOCLAW_LOG` | `info` | Log level: `debug` / `warn` / `error` |
| `GOCLAW_USE_TUI` | — | Set to `1` to use fullscreen TUI (same as `--tui`) |
| `GOCLAW_USE_READLINE` | — | Set to `1` to force readline and disable TUI |
| `GOCLAW_IDE_NOTIFY_URL` | — | Optional `http://127.0.0.1` / `localhost` / `::1` URL for post-tool JSON POST (see IDE notifications above) |

### Settings Files

goclaw merges config from multiple sources in order (later overrides earlier):

```
Built-in defaults → ~/.goclaw/settings.json → .goclaw/settings.json
                  → ~/.goclaw/settings.local.json → .goclaw/settings.local.json
```

`settings.local.json` files are machine-local — do not commit them.

### Example settings.json

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
  "mcp_servers": [
    {
      "id": "demo",
      "command": "node",
      "args": ["path/to/mcp-server.js"]
    }
  ],
  "trusted_workspace": false,
  "external_hooks": [
    {
      "event": "SessionStart",
      "command": "/usr/bin/true"
    }
  ]
}
```

Optional **`bash_timeout_sec`**: positive seconds (capped at 3600) for the bash tool; omit to use the default 30s.

Set **`trusted_workspace`** to `true` only in trusted repos so goclaw loads `.goclaw/hooks.json`. **`external_hooks`** entries use `event` plus either `command` (+ optional `args`) or `url` for HTTP POST (URL must pass the same SSRF rules as `web_fetch` — no loopback).

## CLI (Cobra)

The binary uses [Cobra](https://github.com/spf13/cobra). **CLI flags and subcommands** are separate from **slash commands** (the `/…` REPL verbs in the next section).

Running `goclaw` with no subcommand starts the interactive REPL. Persistent flags apply to that default command:

| Flag | Description |
|------|-------------|
| `--profile <name>` | Agent profile: `general-purpose`, `explore`, `plan`, `verification`, `guide`, `statusline` |
| `--session <id>` | Resume a previously saved session by ID |
| `--list-sessions` | Print saved session IDs and exit |
| `--no-tools` | Chat-only mode — no tools registered |
| `--tui` | Fullscreen Bubble Tea TUI (overrides readline; same as `GOCLAW_USE_TUI=1`) |
| `--readline` | Force readline REPL; disables TUI even if `GOCLAW_USE_TUI=1` |

| Subcommand | Description |
|------------|-------------|
| `goclaw sessions list` | Same as `--list-sessions` |

### Plan file (`.goclaw/plan.md`)

Use **`--profile plan`** (or `/profile plan` in the REPL) to draft a numbered Markdown plan, then save it under **`.goclaw/plan.md`** in your workspace. Commands:

- **`/plan path`** — print the default file path
- **`/plan init`** — create `.goclaw/plan.md` from a template if missing
- **`/plan template`** — print the template on stdout
- **`/apply-plan`** — switch to **`general-purpose`**, load the plan (default path above or an optional path), and run **one** orchestrator turn so the model starts executing the plan (continue with normal messages afterward)

See [AGENT_PROFILES.md](../AGENT_PROFILES.md) and [docs/D16_COORDINATOR_SKETCH.md](docs/D16_COORDINATOR_SKETCH.md) for workflow context and future coordinator mode.

## Slash Commands

**Slash commands** (e.g. `/help`, `/memory`) are handled locally in the REPL and are never sent to the model.

| Command | Description |
|---------|-------------|
| `/help`, `help`, `?` | Show this command list |
| `/session` | Show current session ID and message count |
| `/sessions` | List saved session IDs (same as `goclaw sessions list` / `--list-sessions`, no restart needed) |
| `/quit`, `/exit` | Save session to disk and exit |
| `/new` | Save current session, start a fresh empty session |
| `/save` | Write current session JSONL without exiting |
| `/compact` | Force context compaction — older turns summarized, recent tail kept |
| `/profile <name>` | Switch agent profile without restarting (`general-purpose`, `explore`, `plan`, …) |
| `/plan path` | Print default workspace plan path (`.goclaw/plan.md`) |
| `/plan init` | Create `.goclaw/plan.md` from template if it does not exist |
| `/plan template` | Print the recommended plan Markdown skeleton |
| `/apply-plan [path]` | Load plan file, switch to `general-purpose`, run one execution turn |
| `/memory list` | List memory files under `~/.goclaw/memory/` |
| `/memory add <type> <name> <text...>` | Create a memory entry; types: `user`, `feedback`, `project`, `reference` |
| `/memory delete <file.md>` | Remove one memory file (use `list` to see filenames) |
| `Ctrl+C` | Exit (session is saved on shutdown) |

## Development & Testing

```bash
# Lint
go vet ./...

# Unit tests
go test ./...

# With race detector (requires CGO; on Windows use WSL or Linux CI)
go test -race ./...

# Tests without an API token — point at the mock server
ANTHROPIC_BASE_URL=http://localhost:PORT go test ./...
```

On Windows, `go test` may compile and run temporary `*.exe` test binaries as part of the normal toolchain behavior. These artifacts are not meant to be committed and are ignored by `goclaw/.gitignore`.

The mock server lives in `testutil/mockserver/` and is used by `internal/orchestrator/*_test.go`. It handles Anthropic-format SSE streaming without a real API key.

For a quick **non-interactive smoke** checklist (`--list-sessions`, `--no-tools` expectations, TTY readline checks), see [CLAUDE.md — Non-interactive smoke](CLAUDE.md#non-interactive-smoke).

CI runs `go vet ./...` and `go test -race ./...` on pushes/PRs that touch `goclaw/`. Workflow lives at the monorepo root: [`.github/workflows/goclaw-ci.yml`](../.github/workflows/goclaw-ci.yml) (GitHub only loads workflows from the repository root).

## Documentation

- [`CLAUDE.md`](CLAUDE.md) — authoritative rules: architecture decisions D1–D22, package layout, coding conventions, environment variables, roadmap
- [`../AGENT_PROFILES.md`](../AGENT_PROFILES.md) — built-in profile details, tool filtering, v2+ roadmap
- [`../HOOKS.md`](../HOOKS.md) — hook event system: implemented events, handler registration, v2+ plans
- [`../RETRY_LOGIC.md`](../RETRY_LOGIC.md) — HTTP retry behavior, parameters, per-call budget
- [`../TOOL_CONTRACT.md`](../TOOL_CONTRACT.md) — tool output limits, SSRF policy, loop budgets, MCP tool naming
- [`../MCP.md`](../MCP.md) — MCP reference + goclaw stdio client scope
- [`../IDE_BRIDGE.md`](../IDE_BRIDGE.md) — IDE vs remote bridge; `GOCLAW_IDE_NOTIFY_URL` minimal notifier
- [`../DOCS_MAP.md`](../DOCS_MAP.md) — documentation navigation index
- [`docs/D16_COORDINATOR_SKETCH.md`](docs/D16_COORDINATOR_SKETCH.md) — pre-code sketch for hub-and-spoke coordinator (D16)
