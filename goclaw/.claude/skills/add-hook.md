---
name: add-hook
description: Use when the user asks to add a hook, register a lifecycle handler, or react to tool/session events in goclaw.
---

## Adding a hook in goclaw

### Read first
- `internal/hooks/hooks.go` — `EventType`, `Event`, `Handler`, `Registry`
- `internal/hooks/wire.go` — JSON payload for subprocess/HTTP hooks (`failure_kind` on failures)
- `internal/orchestrator/tool_exec.go` — when `PostToolUse` vs `PostToolUseFailure` fires

**Scope:** For behavior changes to hooks, touch the files above and keep [`docs/reference/hooks.md`](../../../docs/reference/hooks.md) in sync. Do not stop at a single skill or unrelated markdown file.

### Event types

```go
hooks.PreToolUse         // before tool runs (may block)
hooks.PostToolUse        // after success
hooks.PostToolUseFailure // after tool error path
hooks.SessionStart
hooks.SessionEnd

// Future / documented only:
// permission_request, pre_compact, post_compact, subagent_start
```

### Handler examples

```go
func logToolUse(ctx context.Context, e hooks.Event) error {
    slog.Info("tool executed", "tool", e.ToolName, "input", e.Input)
    return nil // nil = do not block
}

func blockDangerousBash(ctx context.Context, e hooks.Event) error {
    if e.ToolName != "bash" {
        return nil
    }
    if strings.Contains(e.Input, "rm -rf") {
        return fmt.Errorf("hook blocked dangerous bash pattern")
    }
    return nil
}
```

### Post-tool failure (`PostToolUseFailure`)

- `e.FailureKind == hooks.FailureExecuteError` — `Tool.Execute` returned a non-nil Go `error` (`Output` may be empty or partial depending on the tool).
- `e.FailureKind == hooks.FailureErrorResult` — `Execute` succeeded but `Result.IsError` was true (`Output` is the tool error payload shown to the model).

Subprocess/HTTP hooks receive the same classification as JSON field `failure_kind` (see `wire.go`).

### Register in `PrepareChatRuntime` (`chat_wiring.go`)

Default hook registration for the REPL/runtime is in [`internal/app/chat_wiring.go`](../../internal/app/chat_wiring.go) inside `PrepareChatRuntime`, not in `cmd/goclaw/main.go`. Pattern:

```go
hookReg := hooks.New()
hookReg.On(hooks.PreToolUse, logToolUse)
hookReg.On(hooks.PreToolUse, blockDangerousBash)
```

### Behavior on Error

| Hook | Non-nil error |
|------|----------------|
| `PreToolUse` | Blocks tool; orchestrator may surface as tool error |
| `PostToolUse` | **`Registry.Fire` logs with `slog.WarnContext`** (`event`, `tool`, `err`); continuation is best-effort |
| `PostToolUseFailure` | Same as `PostToolUse` — logged via `slog.WarnContext`; best-effort |
| `SessionStart` / `SessionEnd` | Logged; session flow continues |

Handlers run in registration order. `PreToolUse` should avoid irreversible side effects when possible.

### Verify
```bash
go build ./...
```
