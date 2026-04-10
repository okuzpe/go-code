# Remote MCP tokens and threat notes

This document complements [MCP.md](../reference/mcp.md) and [CLAUDE.md](../CLAUDE.md) D6 for **enterprise-style** remote MCP usage.

## Implemented (goclaw)

- **`mcp_servers[].bearer_token_file`:** path to a file whose contents (trimmed) are sent as `Authorization: Bearer <token>` when opening a **streamable HTTP** MCP session. Existing `headers` win: if an `Authorization` header is already set (any casing), the file is not applied.
- Paths are resolved relative to the **process working directory** when not absolute (same session as the agent workspace).
- **Store the file with restrictive permissions** (e.g. `chmod 600` on Unix). Do not commit token files.

## Not implemented (explicitly future)

- **OAuth 2.1 / refresh:** interactive login, device code, or headless refresh — needs a dedicated design (secure storage: OS keychain vs encrypted file) and tests with mocks.
- **WebSocket MCP transport:** only streamable HTTP + stdio today; WS would reuse timeout/reconnect ideas from [`internal/llm/retry.go`](../internal/llm/retry.go).
- **Legacy SSE-only HTTP servers:** not supported unless demand is clear.

## Threat model (short)

- **SSRF:** non-loopback `mcp_servers[].url` remains opt-in via `mcp_allow_remote_urls`. Bearer tokens do not reduce SSRF risk; they only authenticate an endpoint the user already chose.
- **Token exfiltration:** a malicious workspace could encourage pointing `bearer_token_file` at sensitive paths; treat token files like API keys.
- **Logs:** avoid logging headers or token file paths beyond debug diagnostics in trusted environments.

## Changelog

| Date | Change |
|------|--------|
| 2026-04-10 | Renamed from `V3_MCP_REMOTE.md`; content unchanged. |
