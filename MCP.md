# MCP (Model Context Protocol)

Depth ties to [ARCHITECTURE.md §2.8](ARCHITECTURE.md). Official spec: [Model Context Protocol](https://modelcontextprotocol.io/specification/2025-11-25). Third-party reference write-up: [MCP — claude-code-explain](https://claude-code-explain.helmcode.com/mcp).

For a coding agent in 2025–2026, built-in tools alone often fall short of user and integrator expectations (GitHub, Slack, browser, databases, etc.). MCP is the **standard adapter** for connecting the model to external processes and services under a shared contract.

---

## Implemented in goclaw (English)

**Code:** [`goclaw/internal/mcp`](goclaw/internal/mcp) — JSON-RPC 2.0 over **stdio** (subprocess) or **Streamable HTTP** ([`goclaw/internal/mcp/http.go`](goclaw/internal/mcp/http.go)): `initialize`, `notifications/initialized`, `tools/list`, `tools/call`. **`mcp_servers`** in merged settings ([`goclaw/internal/config/loader.go`](goclaw/internal/config/loader.go)) uses `command` for stdio, or `url` plus optional `headers` for HTTP. **Loopback-only** URLs unless **`mcp_allow_remote_urls`: true**.

- Tool names exposed to the model: `mcp__<server_id>__<remote_tool>` via `NormalizeMCPToolName`.
- **Client:** hand-rolled `encoding/json` (no official MCP Go SDK required for this layer).
- **Failure isolation:** if a server fails to start or register tools, goclaw logs and skips that server only.
- **Runtime reconnect:** `mcp.ResilientConn` (wired from `PrepareChatRuntime`) re-dials and re-handshakes once when `tools/call` fails with a recoverable transport error (EOF, broken pipe, HTTP 5xx, etc.); then retries the same call.
- **Not implemented:** legacy HTTP+SSE (pre–streamable-HTTP), WebSocket transports, OAuth, resources/prompts as first-class features.

Authoritative module details: [`goclaw/CLAUDE.md`](goclaw/CLAUDE.md) (decision **D6**).

---

## 1. What MCP solves

- **Servers** (subprocess **stdio**, or remote **SSE** / streamable **HTTP** / **WebSocket**) publish **tools**, **resources**, and sometimes **prompts**.
- The **client** (our CLI) keeps sessions, maps model tool calls to `callTool`, applies **permissions** like any other tool, and manages **auth**, timeouts, and output limits.

It does not replace **builtin** tools (`read_file`, `grep`, …); it **complements** them for capabilities you do not want to ship in the binary.

---

## 2. Naming convention exposed to the model

Each MCP tool is exposed to the LLM with a normalized name (Claude Code style):

```text
mcp__<server>__<tool>
```

- Characters outside `[a-zA-Z0-9_-]` → `_`; length capped (e.g. **64** characters in reference).
- Permission rules use these names (including wildcards such as `mcp__slack__*`).

**In goclaw:** pure function `NormalizeMCPToolName(server, tool string) string` shared between `internal/mcp` and `internal/permissions`.

---

## 3. Configuration scope (reference product)

Typical merge order (lowest to highest priority; last writer wins on **server** name conflicts):

| # | Scope | Source (reference) | Notes |
|---|--------|---------------------|-------|
| 1 | org / cloud | Connectors from API | Closed product |
| 2 | plugin | Installed plugins | See [PLUGINS.md](PLUGINS.md) |
| 3 | user | `~/.claude/settings.json` | Global user |
| 4 | project | `.mcp.json` (walk up to home) | **Explicit approval** before connect in reference |
| 5 | local | `settings.local.json` | Not committed |
| 6 | enterprise | `managed-mcp.json` | **Exclusive:** if present, ignores other scopes |

**goclaw today:** `mcp_servers` in `~/.goclaw/settings.json`, project `.goclaw/settings.json`, and `settings.local.json` merge chain — see CLAUDE.md. Remote transports and enterprise policy are **not** implemented.

---

## 4. Transports

| Transport | Typical use | Notes (reference) |
|-----------|-------------|-------------------|
| **stdio** | `command` + `args`; subprocess | Most common; bounded stderr for debug; ~30 s connect timeout |
| **sse** | Remote URL, EventSource | Long-lived stream; HTTP requests ~60 s |
| **http** | Streamable HTTP (spec 2025-03-26) | Session, OAuth |
| **ws** | WebSocket | TLS, proxy |

Config expansion: `${VAR}` and `${VAR:-default}`.

**Future in goclaw:** WebSocket MCP, OAuth, legacy HTTP+SSE-only servers, and non-MCP IDE transports — security review before expanding beyond Streamable HTTP + loopback defaults.

---

## 5. Authentication (reference)

- **OAuth** (SSE/HTTP): browser flow, tokens in secure storage, refresh on 401, revocation RFC 7009.
- **XAA (Cross-App Access):** token exchange (RFC 8693, RFC 7523); one popup may authenticate several servers; often behind feature flags / enterprise.
- **McpAuthTool (pseudo-tool):** if the server fails on auth, expose `mcp__<server>__authenticate`; the model invokes it and the flow runs; in reference it often **auto-approves**; failures cached ~**15 min**.

---

## 6. Lifecycle (summary)

**Startup:** load scopes → dedupe by signature (URL vs command) → filter enterprise policy → approve project servers → connect in parallel → fetch tools / resources / prompts.

**Call:** `tool_use` with name `mcp__…` → ensure connected client → **permissions** → `callTool` with timeout → if output exceeds threshold (~**100k** chars in reference), write to temp and return instruction to read with **read_file**.

**Shutdown:** stdio: staggered SIGINT → SIGTERM → SIGKILL in short windows; remotes: close transport and reject pending.

---

## 7. Permissions and policy

- In reference products, MCP is a **passthrough** to the global permission system: interactive asks, bypass auto-approve, auto-mode may use a classifier (**D17**).
- User rules: `allow` / `deny` with `mcp__…` prefixes.
- Enterprise: `allowedMcpServers` / `deniedMcpServers` (name, URL pattern, command); **deny wins**.

---

## 8. Roadmap (reference vs goclaw)

| Phase | MCP scope |
|-------|-----------|
| **Early MVP (generic)** | Optional: stabilize loop + builtin tools only. |
| **goclaw (shipped)** | `internal/mcp`: **stdio** client, dynamic `mcp__*` tools, output limits, **permissions**, `mcp_servers` config. |
| **Future (goclaw v3+)** | WebSocket, OAuth, resources/list/read, prompts as commands, merge with **plugins** (**D20**) — per **D6** when scoped.

**D6** should still fix: which transports ship when, `.mcp.json` compatibility, and enterprise policy parity if required from day one.

---

## 9. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Initial doc: naming, scopes, transports, auth, lifecycle, permissions, v2/v3 roadmap, **D6**. |
| 2026-04-07 | Added pointer to `internal/mcp`, `mcp_servers`, stdio-only scope. |
| 2026-04-08 | Full English rewrite; **Implemented in goclaw** section first; reference sections retained for future work. |
