# Manual TUI checklist (Tier 1a)

Run from a real TTY with `goclaw` (**TUI is default** on a TTY) or explicitly `goclaw --tui` / `GOCLAW_USE_TUI=1`. Confirm after each step.

1. **Launch and input** — App starts; type a message; streaming assistant output appears without raw JSON tool dumps (compact tool lines + footer status).
2. **Approval modal** — With a tool in `ask` mode, trigger a tool call; modal shows human-readable summary; `y` / `yes` allows, `n` denies.
3. **Ctrl+L** — Clears the viewport; no panic; pending tool queue cleared per product design.
4. **Ctrl+C** — Session saves cleanly on exit (check `~/.goclaw/sessions/` for JSONL or use `/session` after restart).
5. **Slash autocomplete** — On one line, type `/` then a few letters; a filtered command list appears above the input. **Tab** completes; narrow the list by typing more.
6. **`/help` overlay** — Run `/help` (or `help` / `?`); a bordered help panel replaces the transcript area. **Esc** closes and restores the chat view; **↑↓** / **PgUp**/**PgDn** scroll if needed.

## First-run onboarding (Tier 0)

Use a **disposable user config** so you do not wipe your real `~/.goclaw/` (examples: temporary `HOME`, or move/rename `~/.goclaw/settings.json` for one run only).

1. **Fresh settings + TTY** — With no `~/.goclaw/settings.json`, run `goclaw` (or `go run ./cmd/goclaw`) from a project directory. Complete: security screen → trust workspace (option 1) → appearance → Ollama defaults (Enter). Confirm `~/.goclaw/settings.json` exists and project `.goclaw/settings.json` has `"trusted_workspace": true`. Completion tip should mention **`coordinator`** (default hub) vs **`/profile general-purpose`** or **`/profile builder`** for direct coding in the main session.
2. **Readline onboarding** — Same fresh config; run with `GOCLAW_USE_TUI=0` or `--readline`. Walk through the same logical steps; stdin prompts match the TUI flow.
3. **Decline trust** — Choose “exit / not trusted”; process exits without writing user settings (or exits early per implementation); no hang.
4. **Skip wizard** — `GOCLAW_NO_ONBOARDING=1` with missing `settings.json`: app should **not** show the wizard (may still fail later if provider misconfigured).
5. **`goclaw doctor`** — With no `settings.json`, `doctor` should still run (defaults) and **not** run the onboarding wizard.

## Readline history (Tier 1b)

Run without `--tui`. Type a distinctive line, exit cleanly, restart `goclaw`: press ↑ — the line should appear in history (file under `~/.goclaw/history`).

## Automated pre-release gate (maintainers)

Tier 0 / Tier 5 parity is enforced in CI by [`.github/workflows/goclaw-ci.yml`](../../.github/workflows/goclaw-ci.yml): `go vet ./...`, mock parity harness (`TestMockParityHarness`), `go test` (with `-race` and coverage threshold on Linux), `golangci-lint`, `go run ./cmd/goclaw --version`, and stdin mock smoke on Ubuntu.

| Date | Check | Result |
|------|---------|--------|
| 2026-04-11 | `go vet ./...` from `goclaw/` | Pass |
| 2026-04-11 | `go test ./internal/orchestrator ./internal/coordinator -count=1 -run '^TestMockParityHarness$'` | Pass (same target as CI “Mock parity harness”) |
| — | Steps in **First-run onboarding** + **Launch and input** above + `goclaw doctor` on a representative machine | Human sign-off before calling the release fully verified |

## Changelog

| Date | Change |
|------|--------|
| 2026-04-10 | Renamed from `MANUAL_TUI_CHECKLIST.md`; former filename remains a redirect stub. Slash autocomplete + `/help` overlay checklist steps. |
| 2026-04-10 | Added **First-run onboarding (Tier 0)** — fresh config, TUI vs readline, decline trust, `GOCLAW_NO_ONBOARDING`, `doctor` vs wizard. |
| 2026-04-11 | **Automated pre-release gate** table + CI pointer for release `v1.3.0`. |
