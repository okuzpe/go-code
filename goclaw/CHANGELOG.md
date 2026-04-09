# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) where practical.

## [Unreleased]

### Added

- Coordinator `stop_task` tool to cancel in-flight `spawn_agent` workers via `task_id` from spawn results.
- Session list metadata: `/sessions` shows RFC3339 modification time per saved session.
- Memory list previews: first 80 runes of entry body in `/memory list`.
- `/compact` confirmation includes message counts before and after compaction.
- Non-blocking Ollama reachability warning at startup (`internal/app/ollama_probe.go`).
- TUI footer shows elapsed seconds while a tool is running.
- Tests for hooks (HTTP JSON body, stdin JSON for commands, exit 1), MCP registration naming, read-only MCP execution, MCP start failure tolerance, trusted project hooks, race workflow notes, and manual TUI checklist doc.
- GoReleaser configuration for cross-platform archives (`.goreleaser.yaml`).
- MCP `Conn` interface (`internal/mcp/conn.go`) — clean abstraction over stdio and HTTP transports.
- MCP Streamable HTTP client (`internal/mcp/http.go`): `HTTPSession` implements `Conn` for `mcp_servers[].url`; loopback-only by default, opt-in remote via `mcp_allow_remote_urls`; SSE + JSON response modes.
- `ResilientConn` (`internal/mcp/resilient.go`): one-shot reconnect on recoverable transport errors (EOF, broken pipe, reset); transparent to callers.
- IDE lockfile discovery (`internal/ide/discovery.go`): reads `~/.goclaw/ide/*.json` for MCP HTTP endpoints; used when `ide_bridge_mcp` is set.
- `internal/text` package: `TruncateRunes` for rune-safe string truncation (extracted from `ui/chat`).
- Config: `mcp_allow_remote_urls` setting to permit non-loopback MCP HTTP endpoints (default off).
- Config: `ide_bridge_mcp` setting to auto-discover editor MCP endpoint from `~/.goclaw/ide/*.json` lockfiles.
- Config: `MCPServerConfig` now accepts `url` and `headers` fields for Streamable HTTP servers.
- `doctor` / `/doctor` now shows HTTP MCP connection hints (SSRF policy, HTTPS loopback certificate, unauthorized errors).

### Changed

- `spawn_agent` JSON results include `task_id` for use with `stop_task`.
- Clearer MCP `tools/call` errors when the server connection drops mid-request.
- `mcp.ToolAdapter` and `RegisterSessionTools` accept the `mcp.Conn` interface instead of `*mcp.Session`, enabling HTTP and stdio transports interchangeably.
- `chat.go`: `truncateRunes` extracted to `internal/text.TruncateRunes`; local copy removed.
