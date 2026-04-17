---
name: terminal-rendering
description: Use when adding or changing goclaw terminal output, TUI screens, non-TTY banners, Glamour markdown, or Lip Gloss styles. Ensures ui_appearance theme parity with the chat TUI.
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

# Terminal rendering & theme (goclaw)

## Read first

- Optional **minimal cyberpunk** terminal direction: `.cursor/skills/terminal-cyberpunk-aesthetic/SKILL.md`
- Cursor rule: `.cursor/rules/terminal-rendering.mdc`
- TUI structure (Bubble Tea / Bubbles / Lip Gloss): `.cursor/rules/bubbletea-bubbles-lipgloss.mdc` and skill `bubbletea-bubbles-lipgloss`
- Chat theme mapping: `internal/ui/chat/theme_appearance.go`
- Shared Glamour options (no `app` → `chat` import): `internal/config/glamour_opts.go` — `config.GlamourTermRendererOptions(uiAppearance, wordWrap)`

## Checklist

1. Fullscreen UI → Bubble Tea v2 + Bubbles v2; styles → Lip Gloss (and v2 compat in chat as needed).
2. Markdown in the terminal → Glamour; pass **`config.GlamourTermRendererOptions(cfg.UIAppearance, wrap)`** unless you are inside `package chat` using `Theme.RenderMarkdown`.
3. Do not import `internal/ui/chat` from `internal/app` for theming — use `config.GlamourTermRendererOptions`.
4. Non-TTY → plain text fallback.
5. Lip Gloss accents → align with `banner.go` / `Theme` (purple AI, emerald user, muted separators).
