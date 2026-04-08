# Hooks

Hooks are event callbacks that fire at fixed points in the agent loop. They allow inspection or blocking of tool use and session lifecycle events.

---

## §0 — What goclaw implements today

| Mechanism | Where | Notes |
|-----------|--------|--------|
| **Go handlers** | `hooks.Registry.On(...)` | Registered from code (same process). |
| **Settings: `external_hooks`** | `settings.json` / merge chain | Each entry: `event`, plus `command` (+ optional `args`) **or** `url` for HTTP POST. Wired in [`goclaw/internal/app/run.go`](goclaw/internal/app/run.go) (`RunChat`). |
| **Project file** | `.goclaw/hooks.json` | Loaded only when **`trusted_workspace`** is `true` in merged config (`internal/hooks/load.go`). |

Subprocess hooks receive a JSON payload on **stdin** (see `internal/hooks/wire.go`). **Exit code 2** on `PreToolUse` blocks the tool and surfaces an error to the model. Other non-zero exits are logged; for `PreToolUse`, exit codes other than 2 still fail the hook and block.

HTTP hooks use **POST** with `Content-Type: application/json`. URLs must pass **`tools.ValidateWebhookURL`** — same SSRF posture as `web_fetch` (loopback and private ranges blocked).

---

## Implemented in goclaw

Implementation: [`goclaw/internal/hooks/hooks.go`](goclaw/internal/hooks/hooks.go), [`external.go`](goclaw/internal/hooks/external.go), [`load.go`](goclaw/internal/hooks/load.go).

### Events

| Event | When it fires | Blocking? |
|-------|--------------|-----------|
| `PreToolUse` | Before a tool executes | Yes — Go handler error, subprocess exit **2**, or HTTP 4xx/5xx / network error cancels the tool call |
| `PostToolUse` | After a tool succeeds | No — errors logged with `slog.WarnContext` |
| `PostToolUseFailure` | After a tool returns an error | No |
| `SessionStart` | REPL startup, before the first prompt | No |
| `SessionEnd` | REPL shutdown, before saving the session | No |

### Go handler registration

```go
reg := hooks.New()
reg.On(hooks.PreToolUse, func(ctx context.Context, e hooks.Event) error {
    // e.ToolName  — name of the tool about to execute
    // e.Input     — JSON input string passed to the tool
    return nil // return an error to block execution
})
reg.On(hooks.PostToolUse, func(ctx context.Context, e hooks.Event) error {
    // e.Output — JSON output from the tool (PostToolUse only)
    return nil // error is logged but not fatal
})
```

The `Event` struct:

```go
type Event struct {
    Type     EventType
    ToolName string
    Input    string // JSON input passed to the tool
    Output   string // JSON output (PostToolUse only)
}
```

### Settings example (`external_hooks`)

```json
{
  "trusted_workspace": true,
  "external_hooks": [
    { "event": "SessionStart", "command": "/path/to/hook.sh" },
    { "event": "PostToolUse", "url": "https://example.com/hook" }
  ]
}
```

### Project hooks (`.goclaw/hooks.json`)

JSON array of objects with the same shape as `external_hooks` entries. **Disabled** unless `trusted_workspace` is true after settings merge.

### Blocking vs. non-blocking

`PreToolUse` is the only event where failures block tool execution. All other events are best-effort: handler errors are logged and do not interrupt the REPL.

### REPL integration

`SessionStart` / `SessionEnd` fire around the chat REPL in [`goclaw/internal/app/run.go`](goclaw/internal/app/run.go). Built-in Go handlers are optional; settings and project file hooks are loaded there.

---

## Conceptual model (reference product)

The reference product supports approximately 27 named events across tool, session, permission, context, sub-agent, and MCP lifecycles. It has additional handler transports (LLM prompt evaluation, multi-turn agent, async `asyncRewake`, etc.). Those are **not** implemented in goclaw; they remain design targets for future work.

---

## §9 — Security notes

Subprocess hooks run with the **user’s OS permissions** — there is no sandbox. A malicious `.goclaw/hooks.json` in a repository could execute arbitrary code if you set `trusted_workspace: true`.

**Mitigations in goclaw:**

- Project hooks load only when **`trusted_workspace`** is explicitly enabled in merged settings.
- Subprocess hooks use a bounded timeout (`internal/hooks/external.go`).
- HTTP hook URLs are validated against SSRF rules (`internal/tools/ssrf.go`).

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: event list, transport types, source priority, permissions integration, async, security, Go design. |
| 2026-04-07 | Cross-links to CUSTOM_AGENTS.md hooks frontmatter. |
| 2026-04-08 | Added goclaw implementation status (`internal/hooks`); MVP roadmap row aligned. |
| 2026-04-08 | Translated to English; restructured around implemented vs. roadmap; reference-product analysis condensed. |
| 2026-04-07 | §0 / §9: `external_hooks`, `.goclaw/hooks.json` + `trusted_workspace`, subprocess exit 2, HTTP SSRF, REPL wiring in `internal/app/run.go`. |
