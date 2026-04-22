---
name: tui-live-animations
description: >-
  Implement live animated states in goclaw Bubble Tea TUIs: spinner integration,
  phase-driven rendering, animated status bars, visual rulers, and tool timing.
  Use when adding or fixing spinner animations, phase state machines, live agent
  status indicators, or per-frame rendering performance in any goclaw TUI.
---

> **Language:** Author and maintain this file in English only.

## Read first

- `.cursor/rules/tui-live-animations.mdc` — authoritative checklist
- `.cursor/rules/bubbletea-bubbles-lipgloss.mdc` — layer responsibilities
- Reference implementations:
  - `internal/ui/chat/chat_view.go` (`footerPrimaryStatus`, `footerView`)
  - `internal/agentdemo/ui/layout_sync.go` (`agentBar`, `statusLine`)

---

## The core problem this skill solves

Bubble Tea spinner components tick and update their internal frame state, but
**`m.spinner.View()` must be explicitly called in `View()` or a render helper**
to surface the animated character. If you only call `m.spinner.Update(msg)` in
`Update()` and never call `.View()`, the spinner animates silently and the user
sees nothing.

**Wrong — spinner active but invisible:**
```go
// Update: spinner advances internally
m.spinner, cmd = m.spinner.Update(msg)

// View: static glyph — spinner frame discarded
status := th.FooterDim.Render("~") + " " + label
```

**Correct — frame rendered where the user can see it:**
```go
// View: live braille character from the spinning model
status := m.spinner.View() + " " + label
```

---

## Phase state machine

Define phases as typed constants, not bare strings:

```go
type Phase string

const (
    PhaseIdle      Phase = "idle"
    PhaseThinking  Phase = "thinking"
    PhaseStreaming  Phase = "streaming"
    PhaseExecuting Phase = "executing"
)
```

Keep phase transitions explicit in `Update()`:

| Event | Transition |
|-------|-----------|
| User submits | → `PhaseThinking` + reset `lastResult` |
| `streamDeltaMsg` | `PhaseThinking` → `PhaseStreaming` |
| `agentToolStartMsg` | any → `PhaseExecuting` |
| `agentToolDoneMsg` | `PhaseExecuting` → `PhaseThinking` |
| `streamDoneMsg` | any → `PhaseIdle` + set `lastResult` |

---

## Animated status bar (`agentBar` pattern)

One dedicated line between viewport and input, rendered fresh every frame:

```go
func (m *Model) agentBar() string {
    sep := m.st.Dim.Render(strings.Repeat("─", m.width))
    var content string
    switch m.phase {
    case PhaseThinking:
        // Brackets styled independently; spinner.View() concatenated as plain string
        // to avoid double-ANSI-wrapping (spinner already has its accent color).
        content = m.st.Accent.Render("[") + m.spinner.View() + m.st.Accent.Render("]") +
            m.st.Dim.Render(" Thinking...")
    case PhaseStreaming:
        content = m.st.Accent.Render("[▸]") + m.st.Dim.Render(" Writing...")
    case PhaseExecuting:
        content = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[") +
            m.spinner.View() +
            lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("]") +
            m.st.Dim.Render(" Running → " + m.currentTool)
    default:
        switch m.lastResult {
        case "ok":
            content = m.st.Accent.Render("[✓]") + m.st.Dim.Render(" Ready")
        case "error":
            content = m.st.Err.Render("[✖]") + m.st.Dim.Render(" Error")
        default:
            content = m.st.Dim.Render("[·] Ready")
        }
    }
    bar := lipgloss.NewStyle().Width(m.width).Render(content)
    return sep + "\n" + bar
}
```

**Key rules:**
- `agentBarLines = 2` in layout constants (1 separator + 1 status line)
- Update `TranscriptViewportHeight` to subtract `agentBarLines`
- Do NOT wrap `m.spinner.View()` in a v1 Lip Gloss `Render` call — the spinner has its own color from v2

---

## Visual ruler (separator line)

Separate transcript from footer chrome with a full-width rule:

```go
if fw > 0 {
    ruler := th.FooterDim.Width(fw).Render(strings.Repeat("─", fw))
    b.WriteString(ruler)
    b.WriteString("\n")
}
```

Add this at the top of `footerView()` (after special-mode guards, before the status row). The ruler line is counted by `lipgloss.Height(foot)` in `layout()` — no extra constant needed.

---

## Tool execution timing

Track duration per tool call:

```go
// On toolStartMsg:
m.toolStartTime = time.Now()

// On toolDoneMsg:
elapsed := time.Since(m.toolStartTime)
dur := fmt.Sprintf("  %dms", elapsed.Milliseconds())
summary = icon + " " + toolName + dur
```

---

## Performance rules

| Concern | Solution |
|---------|---------|
| O(n) token estimate per frame | Cache `tokenCount`; set `tokensDirty = true` on every block append; recompute only when dirty or streaming |
| Unbounded `blocks` slice | Cap at `maxBlocks = 500`; prune oldest on append |
| Redundant `layout()` calls | Call `layout()` only on resize and input height change; call `syncTranscript()` on block mutations |
| Double `layout()` on submit | `handleSubmit()` already calls it — remove the extra call from the Enter key handler |

---

## Spinner tick discipline

- Return `m.spinner.Tick` from `handleSubmit` when streaming starts (not unconditionally from `Init`)
- In `Update`, forward `spinner.TickMsg` only when active:

```go
case spinner.TickMsg:
    if !m.spinnerActive {
        return m, nil
    }
    var cmd tea.Cmd
    m.spinner, cmd = m.spinner.Update(msg)
    return m, cmd
```

- Set `spinnerActive = false` (or `phase = PhaseIdle`) on `streamDoneMsg` and error paths

---

## Checklist

- [ ] `m.spinner.View()` is called in at least one render path visible to the user
- [ ] Spinner ticks only when a phase is active (`spinnerRunning()` gate)
- [ ] Phase field is a typed constant, not a bare string
- [ ] `agentBar()` does not double-wrap `spinner.View()` in another `Render`
- [ ] Footer has a visual ruler separating it from the transcript
- [ ] Tool timing reported in completed tool summary lines
- [ ] `blocks` slice capped; token estimate cached
- [ ] `go build ./...` clean from module root
