# Prefix input modes (`!`, `@`, `&`, `/btw`, …) — deferred

## Status

**Not implemented.** goclaw uses slash commands (`/help`, `/edit`, …), tools invoked by the model, and the line/TUI input as documented in [usage.md](./usage.md). This document records why a Claude Code–style prefix mini-language is **out of scope** until explicitly specified.

## What other products sometimes do

| Prefix | Typical meaning (elsewhere) | goclaw today |
|--------|----------------------------|--------------|
| `!` | “Bash mode” / send shell to host | No dedicated mode; `bash` tool under permission policy |
| `@` | File path picker / mentions | No picker; `read_file` / `glob` / paste paths |
| `&` | Background task | Coordinator `spawn_agent` + `/focus` / `/detach` |
| `/btw` | Side thread without clearing main context | No equivalent; use `/new` or a separate session |
| `\\` + Enter | Hard newline in single-line UI | TUI: `Ctrl+J` / `Alt+Enter`; readline: configured newline |

## Why this is a separate epic

1. **Security** — A `!` mode that bypasses the normal tool boundary could widen shell exposure unless it reuses the same `bash` policy, approval, and allowlist.
2. **UX contract** — Prefixes interact with multiline input, completion, streaming, and the TUI footer; each needs a defined behavior.
3. **Model vs local** — Clarify what is interpreted locally before send vs what is sent to the model as plain text.
4. **Readline vs TUI** — Feature parity rules (e.g. `@` file completion) differ by frontend.

## If this is picked up later

- Write one short **requirements** section per prefix (trigger syntax, interaction with tools, errors).
- Add **tests** for parsing and for “no accidental shell” regressions.
- Prefer **reusing** existing tools and permissions rather than a parallel execution path.

## Related

- TUI `/` autocomplete and `/help` overlay: [usage.md](./usage.md) § Slash commands, autocomplete, and help.
- Slash command table: `internal/slashcmd/slash_commands.go`.
