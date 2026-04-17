---
name: terminal-cyberpunk-aesthetic
description: >-
  Apply a minimalist cyberpunk visual direction to goclaw terminal and TUI
  surfaces—dark base, sparse neon accents, thin Lip Gloss chrome—while keeping
  Charm Bubble Tea / Bubbles / Lip Gloss and ui_appearance discipline. Use when
  the user asks for cyberpunk, synthwave, neon CLI, futuristic terminal UI, or
  striking dark styling in goclaw.
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

## Read first

- `.cursor/rules/terminal-rendering.mdc` — theme source of truth, Glamour, non-TTY
- `.cursor/rules/bubbletea-bubbles-lipgloss.mdc` — engine vs widgets vs styles
- `internal/ui/terminalstyle/palette.go` — color tokens per `ui_appearance`
- `internal/ui/chat/theme_appearance.go` — chat `Theme` wiring from palette

## Principles

1. **Dark void** — background implied by terminal; surfaces use minimal fill, mostly FG + borders.
2. **One loud accent** — e.g. electric cyan or magenta for AI chrome and interactive highlights; **second accent** only for user / success lines if needed.
3. **Typography beats decoration** — weight and `Muted`/`Dim` carry structure; skip extra glyphs and full-width ornaments.
4. **Readable first** — long assistant text and Markdown body stay high-contrast; neon is for **labels**, **borders**, **pickers**, not paragraphs.

## Where to edit

| Area | Files |
|------|--------|
| Shared TTY colors | `internal/ui/terminalstyle/palette.go` |
| Chat Theme / chips | `internal/ui/chat/theme_appearance.go`, `internal/ui/chat/theme.go` |
| Startup / trust copy | `internal/app/banner.go`, `internal/app/onboarding_trust_styles.go`, `onboarding_render.go` |
| Markdown tone | Glamour style keyed like existing presets; do not hardcode unrelated RGB in `package app` for themed output—use `config.GlamourTermRendererOptions` where applicable |

## Anti-patterns

- Dropping `ui_appearance` or bypassing `PaletteForAppearance` with scattered hex in unrelated packages.
- More than **two** saturated accent hues on the same status row.
- “Cyber” tropes that add noise: fake scanline characters, excessive `▀▄` art, animated-feel clutter in a static TUI.

## Checklist before merge

- [ ] Light and dark (or preset matrix) still legible for core flows.
- [ ] Colorblind / ANSI presets remain usable if touched.
- [ ] `go build ./...`
