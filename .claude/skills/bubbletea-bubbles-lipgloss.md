---
name: bubbletea-bubbles-lipgloss
description: >-
  Build or refactor goclaw fullscreen TUIs: Bubble Tea v2 as the program engine,
  Bubbles v2 for viewport, list, textarea, spinner, and textinput, Lip Gloss for
  layout and styles. Use for new TUI screens, tea.Model wiring, panel styling,
  or when the user mentions Bubble Tea, Bubbles, Lip Gloss, or terminal TUI work
  in goclaw.
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

## Read first

- Cursor rule (authoritative checklist): `.cursor/rules/bubbletea-bubbles-lipgloss.mdc`
- Theme, Glamour, `ui_appearance`, non-TTY: `.cursor/rules/terminal-rendering.mdc`
- Reference implementations: `internal/ui/chat/chat.go`, `internal/app/onboarding_tui.go`, `internal/app/doctor_tui.go`

## Layer responsibilities

| Layer | Owns | Does not own |
|-------|------|--------------|
| **Bubble Tea** | `Init` / `Update` / `View`, `tea.Program`, routing `tea.Msg`, quitting, window size | Per-widget line editing (delegate to Bubbles) |
| **Bubbles** | Stateful widgets (`viewport`, `textarea`, `list`, …) | Global theme policy (see terminal-rendering) |
| **Lip Gloss** | Strings: width, border, padding, foreground/background, join/place | Business logic or message handling |

## Workflow

1. **Root model** — One `tea.Model` (often a pointer type) orchestrates; store Bubbles as struct fields.
2. **Update** — Forward messages to children when appropriate; merge commands with `tea.Batch`.
3. **View** — Compose `child.View()` outputs with Lip Gloss; keep heavy logic out of `View`.

## Repo conventions

- Imports: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`; Lip Gloss package choice must match surrounding code (v2 vs `github.com/charmbracelet/lipgloss` where compat is required).
- **Alt screen** — Configure via Bubble Tea v2 `tea.View` / `tea.NewView` patterns used in chat; do not rely on removed v1-style program options.
- **Receivers** — Use pointer receivers when embedded Bubbles or viewport state mutates across ticks.
- **Non-TTY** — Keep a plain `fmt` (or non-interactive) path when stdin/stdout are not a full terminal; see terminal-rendering rule.

## Quick checklist

- [ ] Single root model coordinates all `tea.Cmd` results.
- [ ] Bubbles updated on every relevant message.
- [ ] `View` only formats and delegates; Lip Gloss for layout.
- [ ] Theme and markdown rendering aligned with `ui_appearance` / `GlamourTermRendererOptions` where applicable.
- [ ] No ad-hoc ANSI for user-visible TUI when Lip Gloss + theme already cover the case.

## Verify

```bash
go build ./...
```

Run from the `goclaw` module root.
