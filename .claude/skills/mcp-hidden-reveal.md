---
name: mcp-hidden-reveal
description: Use when working with MCP tool registration, tool_search, or the hidden/reveal deferral pattern — how MCP tools stay out of the prompt until discovered.
---

> **Language:** English only. Rule: `.cursor/rules/agent-artifacts-english.mdc`.

## MCP hidden/reveal deferral pattern

MCP servers can expose dozens of tools. Sending all of them in every LLM prompt wastes context — especially on 7B local models. goclaw solves this with a **hidden/reveal** split:

1. MCP tools are registered as **hidden** — excluded from the LLM prompt by default.
2. The model calls `tool_search` when it needs a specific capability.
3. Matched hidden tools are **revealed** for the remainder of that turn.

---

### Key components

| Component | File | Role |
|-----------|------|------|
| `Registry.AddHidden` | `internal/tools/registry.go` | Registers a tool as hidden |
| `Registry.IsHidden` | `internal/tools/registry.go` | Reports whether a tool is hidden |
| `Registry.Specs()` | `internal/tools/registry.go` | Returns **visible** tools only (for LLM prompt) |
| `Registry.AllSpecs()` | `internal/tools/registry.go` | Returns all tools (for `tool_search` scoring) |
| `RegisterSessionTools` | `internal/mcp/adapter.go` | Calls `AddHidden` for every MCP tool |
| `effectiveToolSpecs` | `internal/orchestrator/request.go` | Filters hidden tools; passes through revealed ones |
| `revealToolSearchMatches` | `internal/orchestrator/tool_exec.go` | Parses `tool_search` output and marks tools revealed |
| `revealedToolNames` | `internal/orchestrator/user_turn_context.go` | Per-turn set of revealed hidden tools |

---

### Flow

```
RegisterSessionTools → reg.AddHidden(adapter)
                              ↓
buildRequest → effectiveToolSpecs
    Pass 1: drop hidden tools unless:
        a. tool name is in o.ut.revealedToolNames, OR
        b. profile allowlist explicitly names the tool (or wildcard prefix)
    Pass 2: apply profile allowlist (standard filter)
                              ↓
model calls tool_search("need echo from demo server")
                              ↓
revealToolSearchMatches parses result lines "- mcp__demo__echo: ..."
    validates name with isValidToolName (lowercase/digits/underscore/hyphen only)
    checks reg.IsHidden(name) before adding to revealedToolNames
                              ↓
next LLM iteration: effectiveToolSpecs includes mcp__demo__echo
```

---

### Adding a new hidden tool type

To mark a non-MCP tool as hidden (e.g. a rarely-used built-in):

```go
// Instead of reg.Register(myTool):
if err := reg.AddHidden(myTool); err != nil {
    return fmt.Errorf("register hidden tool: %w", err)
}
```

The tool becomes discoverable via `tool_search` but won't inflate the prompt on every turn.

---

### Profile allowlist bypass

A profile can force-expose specific hidden tools by naming them in `ToolAllowlist`:

```json
{
  "tool_allowlist": ["mcp__myserver__*"]
}
```

Wildcards use trailing `*` (prefix match). This is evaluated in `effectiveToolSpecs` Pass 1.

---

### Rules

- **Never call `reg.Register` for MCP tools** — always `reg.AddHidden` (done in `RegisterSessionTools`).
- **`tool_search` must use `AllSpecs()`** not `Specs()` — otherwise hidden tools are invisible to search.
- **Name validation in `revealToolSearchMatches`**: only `[a-z0-9_-]` chars are accepted. Reject anything else before looking it up in the registry.
- **`revealedToolNames` is initialized eagerly** in `turn_loop.go` (`make(map[string]bool)`) — never nil when a turn is active.
- Revealed state is **per-turn only** — it resets when `o.ut` is cleared at turn end.
