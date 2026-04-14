# Tool contract (goclaw)

**Status in goclaw:** **Shipped** — builtin tools and caps match this contract in [`goclaw/internal/tools/`](../../goclaw/internal/tools/); see also decision **D12** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md).

**Purpose:** canonical reference for tool names, risk levels, input/output limits, and network policy as exposed to the LLM. Implemented in `internal/tools/`. Output limits are defined in [`goclaw/internal/tools/limits.go`](../../goclaw/internal/tools/limits.go).

---

## Tools

| Tool (name sent to model) | Risk | Input (summary) | Output cap | Notes |
|---------------------------|------|-----------------|------------|-------|
| `read_file` | `read_only` | `path`; optional `offset_lines` / `limit_lines` | **512 KiB** or **400 lines** (whichever applies first) | Symlinks resolved; path outside workspace → error |
| `glob` | `read_only` | `pattern` (basename or path with `/` for `path.Match`) | **500** paths max; paths use `/` separator | No `..` in pattern; walk starts at workspace root |
| `grep` | `read_only` | `pattern` (regexp); optional `path` (file or directory) | **200** matches total; **512 KiB** read per file; binary files skipped | Workspace-scoped only |
| `write_file` | `write` | `path`, `content` | — | Workspace-scoped; parent dir must exist; atomic write (temp+rename); **1 MiB** content cap; stripped from ReadOnly profiles |
| `edit_file` | `write` | `path`, `old_string`, `new_string`; optional `replace_all` | — | str_replace style; exact match required unless `replace_all:true`; atomic; preserves file mode; **1 MiB** result cap; stripped from ReadOnly profiles |
| `patch` | `write` | `path`, `diff` (git/unified; one file) | **1 MiB** diff; **1 MiB** result | `path` must match `a/` / `b/` headers; binary rejected; stripped from ReadOnly profiles |
| `bash` | `shell` | `command`; optional `cwd` | **stdout+stderr 256 KiB** truncated | D4: expanded binary allowlist (build, git, curl, …) + user confirmation in Ask mode; **single simple command only** — no pipes, `;`, `&&`, redirects, subshells, or `$(...)` (quote-aware scan; quote URLs with `&`); default **30 s** timeout, overridable via `bash_timeout_sec` in settings (1–3600) |
| `script` | `shell` | `script`, optional `cwd` | **stdout+stderr 256 KiB** truncated | Full shell (pipes, `&&`, OR chains, redirections, multi-line). **Registered by default**; disable with `allow_script: false` in settings. Uses same timeout as `bash` (`bash_timeout_sec`). Risk score **70** (YOLO default prompts). |
| `web_search` | `network` | `query` | **8** results max; snippet **2 KiB** each | Backend from `web_search_backend` in settings: **`ddg`** (default), **`brave`**, or **`serpapi`** (API keys via settings or env); optional DDG fallback when using a non-DDG backend (`web_search_fallback_ddg`). Empty/thin results → message with search URL where applicable |
| `web_fetch` | `network` | `url`; optional `max_bytes` | **1 MiB** default; text/HTML only | SSRF + timeout + redirect re-validation (see Network Policy) |
| `todo_write` | `session_meta` | `merge` (bool), `todos[]` (`id`, `content`, `status`) | **50** items max; **500** runes per `content` | In-memory list; snapshot in system prompt; cleared on `/new`; see [`goclaw/internal/todos`](../../goclaw/internal/todos/store.go) |

Go type names may differ; the contract to the LLM uses the names in the first column.

### Coordinator-only tools (main registry)

When the coordinator profile is active, the main tool registry also exposes:

| Tool | Role |
|------|------|
| `spawn_agent` | Start an isolated worker orchestrator with a chosen profile and task string; returns `WorkerNotification` JSON |
| `stop_task` | Cancel a worker by `task_id` from `spawn_agent` |

See [`docs/goclaw/coordinator.md`](../goclaw/coordinator.md) and [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) (D16).

### MCP-backed tools (`mcp__…`)

Remote MCP tools are exposed under normalized names:

```text
mcp__<server_id>__<remote_tool_name>
```

- Normalization: `internal/mcp.NormalizeMCPToolName` (non-alphanumeric characters → `_`).
- **Invocation timeout:** `MCPToolCallTimeout` in `internal/mcp/adapter.go` (default **60 s** per `tools/call`).
- **Output:** returned as tool result text from the MCP server (same caps and handling as other tools once serialized to the model).
- **Permissions:** exact tool name in `tool_permissions` (default **ask** when omitted).
- **Profiles:** `general-purpose` (`nil` allowlist) includes all MCP tools; closed allowlists can use trailing `*` entries (e.g. `mcp__demo__*`) to include a server’s tools. Read-only profiles **block** `mcp__` tools at execution even if listed.

---

## Network Policy (`web_fetch` and `web_search`)

Implementation: [`goclaw/internal/tools/ssrf.go`](../../goclaw/internal/tools/ssrf.go).

- Only `http` and `https` schemes; `file://`, `gopher://`, and other schemes are rejected.
- **Timeout:** 30 s for `web_fetch`; **15 s** for `web_search` (all backends share the same HTTP client timeout in [`limits.go`](../../goclaw/internal/tools/limits.go) `WebSearchTimeoutSec`).
- **Size cap:** truncate or abort when `max_bytes` is exceeded (see limits table above).
- **Redirects:** max 5 hops; host re-validated against SSRF rules after each hop.
- **SSRF:** block RFC1918 (`10/8`, `172.16/12`, `192.168/16`), loopback (`127/8`), `169.254.169.254` (AWS/GCP metadata), IPv6 loopback (`::1`) and link-local.
- No credentials in URLs; no user session cookies forwarded.

---

## Tool Calling Protocol (D2)

- **Shipped CLI:** the runtime uses **Ollama** `/api/chat` with native `tool_calls` / `tool` results only. Legacy **`anthropic`** / **`openai_compatible`** provider strings in settings are **rejected** at `PrepareChatRuntime`.
- **Tests / mocks:** `internal/llm/openai_compat.go` implements OpenAI-style streaming for `testutil/mockopenai` — not a user-configurable provider.
- **Other providers (reference only):** Anthropic Messages API uses `tool_use` / `tool_result` blocks; not wired in the goclaw binary.
- **Fallback:** orchestrator parses assistant message JSON if native format is unavailable (see D2 in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md)).
- **Ollama-specific:** tool results must use the `tool_name` field on `role: "tool"` messages (not `name`). Wrong field breaks the round-trip — the model prints fake JSON instead of receiving results. See [`goclaw/internal/llm/ollama_wire.go`](../../goclaw/internal/llm/ollama_wire.go).

---

## Loop Budgets

| Resource | Value | Notes |
|----------|-------|-------|
| Max LLM iterations per user message | **32** | Includes turns with `tool_results` |
| Max tool calls per user message | **64** | |
| Retries per LLM HTTP call | up to **10** | See [retry-logic.md](./retry-logic.md) (D22) |

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: initial tool table (read/bash/web_search/web_fetch), network policy, D2, loop budget. |
| 2026-04-08 | `glob`/`grep` added; limits aligned to `limits.go`; `web_search` operational details. |
| 2026-04-08 | Translated to English; SSRF implementation pointer added; Ollama `tool_name` note added. |
| 2026-04-08 | `write_file` and `edit_file` added; `MaxWriteFileBytes` = 1 MiB. |
| 2026-04-08 | `bash`: single-command shell syntax policy, `bash_timeout_sec`, expanded allowlist note. |
| 2026-04-08 | `todo_write`: `session_meta` risk class; caps 50 items / 500 runes per content; pointer to `internal/todos`. |
| 2026-04-07 | MCP tool naming, timeout, permissions, and profile rules. |
| 2026-04-14 | `patch`, `script` (default on); `web_search` backends; D2 Ollama-only shipped; coordinator tools subsection. |
