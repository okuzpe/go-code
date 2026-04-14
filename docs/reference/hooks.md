# Hooks

**Status in goclaw:** Five events; in-process registry, **`external_hooks`** in settings, and project **`.goclaw/hooks.json`** when `trusted_workspace` — see **§0** and **D18** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md).

Hooks are event callbacks that fire at fixed points in the agent loop. They allow inspection or blocking of tool use and session lifecycle events.

---

## §0 — What goclaw implements today

| Mechanism | Where | Notes |
|-----------|--------|--------|
| **Go handlers** | `hooks.Registry.On(...)` | Registered from code (same process). |
| **Settings: `external_hooks`** | `settings.json` / merge chain | Each entry: `event`, plus `command` (+ optional `args`) **or** `url` for HTTP POST. Wired in [`goclaw/internal/app/run.go`](../../goclaw/internal/app/run.go) (`RunChat`). |
| **Project file** | `.goclaw/hooks.json` | Loaded only when **`trusted_workspace`** is `true` in merged config (`internal/hooks/load.go`). |

Subprocess hooks receive a JSON payload on **stdin** (see [`goclaw/internal/hooks/wire.go`](../../goclaw/internal/hooks/wire.go)). Fields: `hook_event_name`, `tool_name`, `tool_input`, `tool_output` (on post-tool events), and optional `failure_kind` on `post_tool_use_failure` (`execute_error` when `Tool.Execute` returned an error, `error_result` when the tool finished with `Result.IsError`). **Exit code 2** on `PreToolUse` blocks the tool and surfaces an error to the model. Other non-zero exits are logged; for `PreToolUse`, exit codes other than 2 still fail the hook and block.

HTTP hooks use **POST** with `Content-Type: application/json`. URLs must pass **`tools.ValidateWebhookURL`** — same SSRF posture as `web_fetch` (loopback and private ranges blocked).

---

## Implemented in goclaw

Implementation: [`goclaw/internal/hooks/hooks.go`](../../goclaw/internal/hooks/hooks.go), [`external.go`](../../goclaw/internal/hooks/external.go), [`load.go`](../../goclaw/internal/hooks/load.go).

### Events

| Event | When it fires | Blocking? |
|-------|--------------|-----------|
| `PreToolUse` | Before a tool executes | Yes — Go handler error, subprocess exit **2**, or HTTP 4xx/5xx / network error cancels the tool call |
| `PostToolUse` | After a tool succeeds | No — handler errors logged with `slog.WarnContext` |
| `PostToolUseFailure` | After `Execute` errors or after a successful `Execute` with `Result.IsError` | No — same logging; payload includes `failure_kind` (`execute_error` vs `error_result`) for subprocess/HTTP |
| `SessionStart` | REPL startup, before the first prompt | No |
| `SessionEnd` | REPL shutdown, before saving the session | No |

### Go handler registration

Built-in `reg.On(...)` wiring for the chat runtime lives in [`goclaw/internal/app/chat_wiring.go`](../../goclaw/internal/app/chat_wiring.go) (`PrepareChatRuntime`), not in `cmd/goclaw/main.go`.

```go
reg := hooks.New()
reg.On(hooks.PreToolUse, func(ctx context.Context, e hooks.Event) error {
    // e.ToolName  — name of the tool about to execute
    // e.Input     — JSON input string passed to the tool
    return nil // return an error to block execution
})
reg.On(hooks.PostToolUse, func(ctx context.Context, e hooks.Event) error {
    // e.Output — tool result body (JSON or text per tool)
    return nil // error is logged but not fatal
})
reg.On(hooks.PostToolUseFailure, func(ctx context.Context, e hooks.Event) error {
    // e.Output — same as post success path; may be empty or partial when failure_kind == execute_error
    // e.FailureKind — hooks.FailureExecuteError or hooks.FailureErrorResult
    return nil
})
```

The `Event` struct:

```go
type Event struct {
    Type        EventType
    ToolName    string
    Input       string // JSON input passed to the tool
    Output      string // Tool result on post_tool_use / post_tool_use_failure (may be partial if Execute errored)
    FailureKind string // post_tool_use_failure only: FailureExecuteError or FailureErrorResult
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

For post-tool events, `Registry.Fire` returns a non-nil error only in exceptional cases (for example JSON marshal failure before external hooks); Go handler errors on `PostToolUse` / `PostToolUseFailure` are logged inside `Fire` and do not appear as the returned `error`.

### REPL integration

`SessionStart` / `SessionEnd` fire around the chat REPL in [`goclaw/internal/app/run.go`](../../goclaw/internal/app/run.go). Built-in Go handlers are optional; settings and project file hooks are loaded there.

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
| 2026-04-07 | Cross-links to custom-agents.md hooks frontmatter. |
| 2026-04-08 | Added goclaw implementation status (`internal/hooks`); roadmap row aligned. |
| 2026-04-08 | Translated to English; restructured around implemented vs. roadmap; reference-product analysis condensed. |
| 2026-04-07 | §0 / §9: `external_hooks`, `.goclaw/hooks.json` + `trusted_workspace`, subprocess exit 2, HTTP SSRF, REPL wiring in `internal/app/run.go`. |
| 2026-04-14 | `Event.FailureKind`, wire field `failure_kind`, `Registry.Fire` return semantics for post-tool events, `Output` semantics on execute errors. |
