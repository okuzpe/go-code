# goclaw — Usage

Task-oriented guide: how to run the CLI, configure providers, and use sessions, tools, and profiles. Deep reference for tools, limits, and architecture lives in [CLAUDE.md](CLAUDE.md) and the monorepo docs linked from [README.md](README.md).

## Quick start (Ollama)

Prerequisite: Ollama on `http://localhost:11434` with a model pulled (default: `qwen2.5-coder:14b`).

```bash
cd goclaw
go run ./cmd/goclaw doctor
go run ./cmd/goclaw              # readline REPL on TTY
go run ./cmd/goclaw --tui       # fullscreen Bubble Tea UI
```

Try: a simple repo question, a tool (e.g. web search), and `/doctor` or `goclaw doctor`.

### REPL modes

- **Readline** — default on TTY; `--readline` or `GOCLAW_USE_READLINE=1` forces it.
- **TUI** — `--tui` or `GOCLAW_USE_TUI=1`.

Exit: `Esc` (TUI) or `Ctrl+C`. Clear: `Ctrl+L`.

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

**Mock regression bundle (no API key):** `make parity` or [scripts/MOCK_PARITY_HARNESS.md](scripts/MOCK_PARITY_HARNESS.md).

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

Full environment variable list: [CLAUDE.md — Environment Variables](CLAUDE.md).

## Agent profiles

Set with `--profile <name>` or `agent_profile` in settings.

| Profile | Value | Tools (summary) | Read-only |
|---------|-------|-----------------|-----------|
| General-Purpose | `general-purpose` | All built-ins + MCP | No |
| Explore | `explore` | read, glob, grep, web, todos | Yes |
| Plan | `plan` | read, glob, grep, web_search, todos | Yes |
| Verification | `verification` | read_file, bash, todos | No |
| Guide | `guide` | none | Yes |
| StatusLine | `statusline` | none | Yes |
| Coordinator | `coordinator` | spawn_agent, stop_task, todo_write | Yes |

Coordinator delegates work to workers; see [docs/D16_COORDINATOR_SKETCH.md](docs/D16_COORDINATOR_SKETCH.md).

## Built-in tools (summary)

| Tool | Role |
|------|------|
| `read_file`, `glob`, `grep` | Read/search workspace |
| `bash` | One simple command; allowlist; timeout (default 30s, override `bash_timeout_sec`) |
| `write_file`, `edit_file` | Writes (stripped on read-only profiles) |
| `web_fetch`, `web_search` | Network (SSRF rules on fetch) |
| `todo_write` | Session task list |
| `spawn_agent`, `stop_task` | Coordinator only — start / cancel isolated workers |

Caps, SSRF, and MCP naming (`mcp__<id>__<name>`): [../TOOL_CONTRACT.md](../TOOL_CONTRACT.md). Workspace path rules: `internal/tools/workspace_paths.go`.

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

Use profile `plan` to draft, save under `.goclaw/plan.md`. In the REPL: `/plan path`, `/plan init`, `/plan template`, `/apply-plan [path]` (loads plan, switches to `general-purpose`, one orchestrator turn). See [../AGENT_PROFILES.md](../AGENT_PROFILES.md).

## Slash commands (REPL)

Handled locally (not sent to the model): `/help`, `/doctor`, `/session`, `/sessions`, `/quit`, `/exit`, `/new`, `/save`, `/compact`, `/profile`, `/plan`, `/apply-plan`, `/memory`. Same health output as `goclaw doctor` when `/doctor` is wired in the REPL.

## Hooks, MCP, IDE ping

- **Hooks:** `PreToolUse`, `PostToolUse`, session start/end; Go API, `external_hooks` in settings, optional `.goclaw/hooks.json` when `trusted_workspace`. [../HOOKS.md](../HOOKS.md)
- **MCP:** `mcp_servers` in settings; stdio and streamable HTTP; optional `bearer_token_file` on URL servers. [../MCP.md](../MCP.md), [docs/V3_MCP_REMOTE.md](docs/V3_MCP_REMOTE.md)
- **Plugins / skills / swarm:** [CLAUDE.md](CLAUDE.md) D20 and “Skills (runtime)” row; [`docs/SWARM.md`](docs/SWARM.md)
- **IDE:** `GOCLAW_IDE_NOTIFY_URL` — localhost POST after each tool (best-effort). [../IDE_BRIDGE.md](../IDE_BRIDGE.md)

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
