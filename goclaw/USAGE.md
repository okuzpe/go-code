# goclaw — Usage

Task-oriented guide for running `goclaw` day-to-day.

## Quick start (local Ollama)

Prerequisite: Ollama running on `http://localhost:11434` with a model pulled (default: `qwen2.5-coder:14b`).

```bash
cd goclaw
go run ./cmd/goclaw --tui
```

Try:

- Ask for something simple like “summarize this repo”.
- Trigger a tool: “search the web for Go 1.26 release notes”.
- Run a health check: `/doctor` (or `goclaw doctor`).

## REPL modes

goclaw supports two interactive UIs:

- **Fullscreen TUI**: `--tui` (or `GOCLAW_USE_TUI=1`)
- **Readline REPL**: default on TTY; `--readline` forces it (or `GOCLAW_USE_READLINE=1`)

Exit: `Esc` (TUI) or `Ctrl+C` (both). Clear: `Ctrl+L`.

## Sessions

Sessions are persisted as JSONL under `~/.goclaw/sessions/<id>.jsonl` when the app exits (or on `/save`).

- **List sessions**:

```bash
go run ./cmd/goclaw --list-sessions
# or
go run ./cmd/goclaw sessions list
```

- **Resume a session**:

```bash
go run ./cmd/goclaw --session <id>
```

## Slash commands (interactive)

Slash commands are handled locally (not sent to the model).

- `/help` — list commands
- `/sessions` — list saved session ids
- `/session` — show current session info
- `/new` — save current session and start a fresh one
- `/save` — persist current session without exiting
- `/profile <name>` — switch profile without restarting
- `/compact` — force context compaction
- `/memory list|add|delete` — manage durable memory
- `/plan init|template|path` and `/apply-plan` — planfile workflow

## Tools, permissions, and what you see in the UI

Tools are visible as compact “agent is working” lines (Claude Code style): no raw JSON is printed, but you can still tell what is happening.

Tool execution is controlled by permissions:

- `ask` (default) — prompt before running
- `allow` — run without prompting
- `deny` — never run

Configure per-tool permissions in `settings.json` (see README for merge order):

```json
{
  "tool_permissions": {
    "read_file": "allow",
    "bash": "ask",
    "web_fetch": "ask",
    "web_search": "ask"
  }
}
```

## Web tools: `web_search` vs `web_fetch`

- **Use `web_search`** when you want discovery (queries). It uses DuckDuckGo’s instant-answer JSON, so for breaking news you may get a thin result plus a suggested search link.
- **Use `web_fetch`** when you already know the URL and want the actual page text (SSRF-protected; text-only).

## Anthropic API (optional)

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

Then set `"provider": "anthropic"` in `~/.goclaw/settings.json` (or `.goclaw/settings.json` in the project).

## Troubleshooting

- **Ollama connection refused**: start the daemon (for example `ollama serve`) or set `OLLAMA_HOST`.
- **Tools feel “empty”**: some tools (like `web_search`) can legitimately return thin results; prefer more specific queries or use `web_fetch` on a known URL.

