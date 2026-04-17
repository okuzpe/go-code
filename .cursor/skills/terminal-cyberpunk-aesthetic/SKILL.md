---
name: terminal-cyberpunk-aesthetic
description: >-
  Guides a minimalist cyberpunk look for goclaw terminal and TUI output—dark
  void, one or two neon accents, thin Lip Gloss structure—without breaking
  ui_appearance or the Charm stack. Use when the user asks for cyberpunk,
  synthwave, neon terminal, futuristic CLI, or bold dark TUI styling in goclaw.
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

# Terminal cyberpunk aesthetic (goclaw)

## Read first

- Stack and theme rules: `.cursor/rules/terminal-rendering.mdc`, `.cursor/rules/bubbletea-bubbles-lipgloss.mdc`
- Palette tokens: `internal/ui/terminalstyle/palette.go`, `internal/ui/chat/theme_appearance.go`
- Non-chat terminal chrome: `internal/app/banner.go`, onboarding styles under `internal/app/onboarding_*.go`

## Design intent (minimal + striking)

| Do | Avoid |
|----|--------|
| Near-black / deep charcoal base; **one primary neon** (cyan or magenta) + optional **second accent** (green or amber) for hierarchy | Rainbow gradients on every line, heavy “hacker” ASCII frames |
| **Thin** borders and rules (`Border`, muted `SepFG`); lots of negative space | Thick padded pills on every label; competing glow-like blocks |
| **Bold** for titles and active state; `Dim` / `Muted` for secondary copy | Neon-colored body paragraphs (unreadable contrast) |
| Align Markdown (Glamour) **dark** style with the same palette family | Glamour theme that fights the Lip Gloss chrome |

Cyberpunk here means **controlled neon on a dark field**, not decorative noise.

## Lip Gloss patterns

- Prefer **foreground accent** over **background fills** for small labels (footer chips, picker names).
- Use `Padding(0,1)` sparingly; prefer margin via layout (`JoinHorizontal` with `strings.Repeat(" ", n))` when separating blocks.
- Modal / input borders: single `BorderForeground` accent; keep `ModalBody` and `DimFG` high enough contrast for long text.

## Palette work

- **Rich terminals:** tune `paletteAuto`, `paletteFixed`, or add a **new `ui_appearance` preset** in `internal/config` + `NormalizeUIAppearance` + `PaletteForAppearance` so users can opt in without breaking defaults.
- **ANSI / colorblind presets:** limit neon to **one** ANSI bright color for emphasis; keep the rest in dim/normal—see existing `paletteANSI` patterns.
- Keep **error** and **success** semantics distinct from decorative accent (do not recolor `ErrorFG` to magenta for “style”).

## Icons (`tui_icons`)

- **ASCII** can read as retro-terminal cyberpunk; **unicode** box-drawing is clean; **emoji** can clash with a stark neon look—pick **one** lane and stay consistent across footer, tool cards, and welcome.

## Verify

- `go build ./...` from the `goclaw` module root.
- Manual TTY pass: welcome, transcript, footer, `/` picker, tool approval strip, modal—readable at a glance.

Do not expand tests unless the user asks.
