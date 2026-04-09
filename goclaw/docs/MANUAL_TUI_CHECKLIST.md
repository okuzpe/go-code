# Manual TUI checklist (Tier 1a)

Run from a real TTY with `goclaw --tui` (or `GOCLAW_USE_TUI=1`). Confirm after each step.

1. **Launch and input** — App starts; type a message; streaming assistant output appears without raw JSON tool dumps (compact tool lines + footer status).
2. **Approval modal** — With a tool in `ask` mode, trigger a tool call; modal shows human-readable summary; `y` / `yes` allows, `n` denies.
3. **Ctrl+L** — Clears the viewport; no panic; pending tool queue cleared per product design.
4. **Ctrl+C** — Session saves cleanly on exit (check `~/.goclaw/sessions/` for JSONL or use `/session` after restart).

## Readline history (Tier 1b)

Run without `--tui`. Type a distinctive line, exit cleanly, restart `goclaw`: press ↑ — the line should appear in history (file under `~/.goclaw/history`).
