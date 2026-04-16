# Prefix input modes (`!`, `@`, `&`, `/btw`)

**Operator summary:** [usage.md — Prefix input](usage.md#prefix-input----btw). This file is the detailed behavior and security reference.

## Status

**Implemented.** These prefixes are interpreted locally in the fullscreen TUI **after** slash commands ([`HandleSlash`](../../goclaw/internal/slashcmd/slash.go)) and **before** the message is sent to the model. They reuse the same **tool registry, permission policy, approval flow, and hooks** as model-invoked tools (see [`RunToolInvocation`](../../goclaw/internal/orchestrator/tool_invocation.go)).

## Dispatch order

1. **Slash** — Lines handled by `/help`, `/memory`, … (not sent to the model unless a command returns `modelSubmit`, e.g. `/btw`, `/edit`).
2. **Prefix** — Parsed by [`internal/inputprefix`](../../goclaw/internal/inputprefix/); may run a single tool locally or rewrite text for the model.
3. **Model** — Remaining text is passed to `RunStreaming` (or to a focused worker when coordinator routing is active).

**Worker focus:** When input is routed to a coordinator worker (`/focus`), prefix modes are **not** applied; the full line is delivered to the worker session.

**Mock (`--mock`):** Prefix interpretation is **disabled**; input is passed through to the mock streamer so UI tests stay deterministic.

## Multiline input (TUI)

- For **`!`** and **`&`**, only the **first line** of the buffer is used; any following lines must be empty after trim, or the parser returns an error (avoids silent truncation of pasted blocks).
- **`@` mixed with extra text** — when `@path` is followed by additional text on the same or subsequent lines (e.g. `@go.mod explain this`), the message is treated as **KindPassthrough**: sent to the model with the file contents silently pre-loaded via `ExpandInlineAtRefs`. Pure standalone `@path` (no extra text) still runs `read_file` locally as before.
- Normal messages may still use **Shift+Enter** / **Alt+Enter** for newlines without these prefixes.

## Per-prefix behavior

| Prefix | Syntax | Effect |
|--------|--------|--------|
| `!` | `!` + shell command (first line) | Runs the **`bash`** tool with JSON `{"command":"…"}`. Same allowlist, metacharacter rules, timeout, and permissions as a model-requested bash call. Output is shown in the UI and recorded in the session as a user line plus a short assistant summary (no fake `tool_use` blocks in history). |
| `@` | `@` + path (standalone line) | Runs **`read_file`** with JSON `{"path":"…"}`. Path resolution matches the workspace-scoped `read_file` tool (relative to workspace or absolute inside it). When `@tokens` appear inside a larger message, `ExpandInlineAtRefs` silently reads each one and prepends the file content as context before the model call. |
| `&` | `&` + task description (first line) | Runs **`spawn_agent`** with `profile: general-purpose`, `task` set to the text after `&`, default `timeout_sec`, `interactive: false`. Requires **`spawn_agent`** on the active profile’s tool registry (the default **`coordinator`** profile includes it; **`general-purpose`** does not — switch profile or use hub mode). If the tool is unavailable, the user sees a clear error. |
| `/btw` | `/btw` + text (slash command) | Handled in [`slash.go`](../../goclaw/internal/slashcmd/slash.go): rewrites to a **single user message** with an explicit “side question” preamble so the model answers briefly without abandoning the main thread. **Session:** the rewritten text is what is stored as the user turn (the `/btw` prefix is not preserved verbatim). |

## Security

- **`!`** must **not** bypass [`rejectShellMetacharacters`](../../goclaw/internal/tools/bash.go), the bash binary allowlist, `Policy.Evaluate`, interactive approval, or `PreToolUse` / `PostToolUse` hooks.
- **`@`** must not read outside the workspace; enforcement is entirely in the **`read_file`** tool implementation.
- **`&`** is subject to **`spawn_agent`** policy (deny/ask/allow) like any other tool call.

## TUI details

- **Parsing:** [`inputprefix.Analyze`](../../goclaw/internal/inputprefix/analyze.go) classifies prefix lines before the model runs.
- **`@` completion:** The fullscreen TUI shows a **strip of matching workspace paths** under the footer (like `/` command hints) and **Tab** completes the longest shared prefix (see `internal/ui/chat`).

## Related

- Slash commands and `/help`: [usage.md](./usage.md).
- Slash command table: `goclaw/internal/slashcmd/slash_commands.go`.
