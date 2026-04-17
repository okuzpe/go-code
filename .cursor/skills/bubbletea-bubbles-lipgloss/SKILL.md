---
name: bubbletea-bubbles-lipgloss
description: >-
  Builds or refactors goclaw fullscreen TUIs using Charm Bubble Tea v2 as the
  program engine, Bubbles v2 for viewport/list/textarea/spinner/textinput, and
  Lip Gloss for borders, colors, and layout. Use when adding a new TUI screen,
  wiring tea.Model, styling panels, or when the user mentions Bubble Tea,
  Bubbles, Lip Gloss, TUI, or terminal UI in goclaw.
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

# Bubble Tea · Bubbles · Lip Gloss (goclaw)

## Read first

- Rule: `.cursor/rules/bubbletea-bubbles-lipgloss.mdc`
- Theme, Glamour, non-TTY: `.cursor/rules/terminal-rendering.mdc` and skill `.cursor/skills/terminal-rendering/SKILL.md`

## Workflow

1. **Bubble Tea (engine)** — Define a root model with `Init`, `Update`, `View`. Start with `tea.NewProgram` and the same TTY options as onboarding/chat when the flow owns the console (`onboardingTeaOptsControllingTTY` is the product reference).
2. **Bubbles (widgets)** — Embed `viewport.Model`, `textarea.Model`, `list.Model`, etc. In `Update`, forward messages to children when appropriate and merge commands with `tea.Batch`.
3. **Lip Gloss (presentation)** — In `View`, build blocks with `lipgloss.NewStyle()`, widths from stored `tea.WindowSizeMsg`, and compose with `JoinHorizontal` / `JoinVertical` / `Place`.

## Repo conventions

- **v2:** `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`; Lip Gloss package must match surrounding code (`lipgloss/v2` vs `github.com/charmbracelet/lipgloss` where compat is needed).
- **AltScreen:** configure on the program `tea.View`; do not rely on removed v1-style options.
- **Keys and quit:** handle in `Update` (q/Esc/Ctrl+C, scroll) and return explicit `tea.Quit`.
- **Non-TTY:** keep a plain `fmt` or non-interactive path per `terminal-rendering`.

## Quick checklist

- [ ] Single root model orchestrates messages and commands.
- [ ] Bubbles receive updates on every relevant tick.
- [ ] `View` only composes strings with Lip Gloss and delegates to `child.View()`.
- [ ] Theme aligned with `ui_appearance` and `terminal-rendering`.
- [ ] Fallback when stdin/stdout are not terminals.
