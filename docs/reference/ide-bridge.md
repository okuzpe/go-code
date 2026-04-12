# IDE integration (local) and Bridge (remote) — reference and Go mapping

**Status in goclaw:** **Partial** — lockfile MCP discovery, `GOCLAW_IDE_NOTIFY_URL`; see **D21** / §6–§7 in this file and [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md).

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (IDE / D21). Reference (third-party, Claude Code analysis): [Bridge & IDE — claude-code-explain](https://claude-code-explain.helmcode.com/bridge-ide).

These are **two distinct systems**. Conflating them in design leads to security confusion (trusted localhost vs. Internet tunnel and OAuth).

**Relation to [mcp.md](./mcp.md):** the IDE integration makes the CLI act as an **MCP client toward the editor** (localhost). **Integration MCP servers** (GitHub, Slack, …) are a different axis: the CLI as **client** toward processes/URLs configured in `mcpServers` — see **D6** in [CLAUDE.md](../../goclaw/CLAUDE.md) and transports in [mcp.md](./mcp.md).

---

## 1. Two systems (not one)

| System | Actors | Transport | Goal |
|--------|--------|-----------|------|
| **IDE integration** | Editor (VS Code, JetBrains, …) and **CLI on the same machine** | **MCP over localhost** (SSE or WebSocket) | Sync edits, diffs, context (selection, open files, cursor) |
| **Bridge** | Browser (e.g. claude.ai), provider backend, **CLI as worker** | HTTPS / WSS / SSE per protocol version | Remote control of the CLI from the web, shared sessions, `can_use_tool` permissions in the browser |

**Priority for our Go assistant:**

- **IDE integration:** treated as a **strong step** after the core REPL (**D21**): many users live in VS Code / Cursor / Windsurf; without this bridge the CLI is isolated from the editing flow.
- **Anthropic-style bridge:** **not** 1:1 parity with the reference product; it implies OAuth identity, third-party infrastructure, and web session semantics. Document as **reference**; a "custom bridge" (web UI + worker) would be a **later phase** and a separate design.

---

## 2. IDE integration (local)

### 2.1 Architecture idea (reference)

- **IDE extensions** start an **MCP server** on `localhost` with an internal transport like **`sse-ide`** (EventSource / SSE) or **`ws-ide`** (WebSocket); sometimes with an **auth token** in the header or query string.
- The **CLI discovers** editor instances by reading **lockfiles** in a directory (in the reference: `~/.claude/ide/<ide-name>-<pid>.lock`) that the IDE creates when the extension starts and deletes on close. Each lockfile carries the **port**, **transport type**, and metadata.
- The CLI acts as an **MCP client** toward that local endpoint to:
  - **Notify edits** (the user accepts/rejects in an editor diff).
  - **Share context** with the agent (selection, open tabs, cursor position).
  - **Open/close diff tabs** (`openDiffInIde`, `closeDiffTabsInIde` in reference).
- There may be an additional **bidirectional channel** (e.g. private notifications between CLI and extension) for feature flags, optional telemetry, or quick actions — it is important to **scope** what is standard MCP vs. private extension (**D21**).

### 2.2 Editors (reference)

- **VS Code / Cursor / Windsurf:** extension that can be installed automatically (`code --install-extension …` in the reference flow).
- **JetBrains (multiple IDEs):** **manual** plugin; same localhost + lockfile discovery idea if the ecosystem standardizes it.

### 2.3 Threat surface (local)

- **Localhost is not "free":** other processes on the same machine may attempt to connect to the IDE port; that is why a **token in `ws-ide`** and constrained binds matter.
- **Diffs and change acceptance** should be linked to the CLI's **permissions** layer ([CLAUDE.md](../../goclaw/CLAUDE.md)): an edit "from the agent" may still go through **Ask** / policies depending on mode.

---

## 3. Bridge (remote) — reference

### 3.1 What it adds over the CLI alone

- The CLI becomes **controlled from the browser**; the worker can live on **another machine** (server, interactive CI, etc.).
- **Remote permissions:** the `can_use_tool` flow sends a control request to the backend; the UI shows Allow/Deny and can **return mutated arguments** or **new session rules** — a pattern useful to copy in a custom UI, **not** dependent on Anthropic.
- **Startup modes** (reference): dedicated multi-session process, bridge inside the REPL, or "no-env" mode with direct session connection; **worktree** to isolate parallel sessions in Git.

### 3.2 Protocol versions (summary)

| Version | Read (incoming to CLI) | Write (outgoing) | Notes |
|---------|------------------------|------------------|-------|
| **v1** | WebSocket | HTTP POST with batching (~100 ms) | Polling/work via "Environments API" in reference; more complex |
| **v2** | SSE | HTTP POST (unified client) | More direct session creation; sometimes behind **feature gate** in reference |

Typical constants (order of magnitude in reference): SSE reconnect backoff 1 s → 30 s, give up ~10 min; liveness ~45 s without data; JWT refresh ~5 min before expiry; up to **3 consecutive** init failures disable the bridge.

### 3.3 Authentication (reference)

Approximate resolution order: environment variable (dev only) → OS credential chain → interactive OAuth. Common headers toward API: `Authorization: Bearer`, API versioning, corporate betas, and at some levels a trusted device token.

**Go mapping:** if a "custom bridge" is ever implemented, **do not** couple the orchestrator core to third-party OAuth; isolate in `internal/remoteui` or similar.

---

## 4. Go mapping (summary)

| Piece | Suggested location | Notes |
|-------|-------------------|-------|
| IDE discovery | `internal/ide/discovery.go` | Scan lockfile directory (path under **D21**, e.g. `~/.goclaw/ide/`); invalidate on PID deletion |
| MCP client → IDE | `internal/ide/mcpclient.go` (or sub-package) | SSE/WS + auth; timeouts; **localhost only** by default |
| Orchestrator contract | Interface injected from `main` | e.g. `IDENotifier`, `IDEContextProvider`; **avoid** `ide` → `orchestrator` |
| Write/Edit tools | After logical success | Call notification for editor diff when **D21** and IDE session is active |
| Remote bridge | **Not implemented** in goclaw; would be a separate package and separate input channel |

**goclaw (shipped) vs reference:** The CLI already includes **REPL/TUI**, permissions, tools, memory, MCP, etc. ([`CLAUDE.md`](../../goclaw/CLAUDE.md)). **D21 / `internal/ide`:** loopback POST notification + lockfile MCP discovery — see §6–§7 below. **Partial:** UI/extension parity with the reference product's IDE. **claude.ai-style bridge:** **no implementation commitment**.

---

## 6. goclaw implementation (minimal)

**Current code:** [`goclaw/internal/ide/notify.go`](../../goclaw/internal/ide/notify.go), [`goclaw/internal/ide/discovery.go`](../../goclaw/internal/ide/discovery.go).

- Environment variable **`GOCLAW_IDE_NOTIFY_URL`**: if set to `http` or `https` and the host is **`127.0.0.1`**, **`localhost`**, or **`::1`**, the orchestrator's **after-tool** callback issues a **best-effort POST** with JSON `{"tool", "result_bytes", "is_error"}` after each tool completes. Remote URLs are rejected (no-op notifier).
- **`ide_bridge_mcp`:** when `true` in merged `settings.json`, goclaw scans **`~/.goclaw/ide/*.json`** (sorted by name), reads the first file with a valid **loopback** `url` and optional `headers`, and appends a synthetic MCP server **`id: "ide"`** before connecting (same Streamable HTTP stack as `mcp_servers[].url` in [`goclaw/internal/mcp/http.go`](../../goclaw/internal/mcp/http.go)). Extensions can drop a lockfile such as `{"url":"http://127.0.0.1:1234/mcp","headers":{"Authorization":"Bearer …"}}`.
- **D21 "full bridge"** still depends on editor-side MCP servers and UX; goclaw provides HTTP client + discovery, not IDE UI.

---

## 7. goclaw ↔ extension contract (V3 spec)

**Goal:** keep editor extensions and goclaw aligned without coupling the orchestrator to a specific IDE.

| Concern | Contract |
|---------|-----------|
| **MCP endpoint** | Extension writes a JSON lockfile under `~/.goclaw/ide/*.json` with `url` (loopback `http`/`https`) and optional `headers`. User sets `ide_bridge_mcp: true`; goclaw appends synthetic MCP server `id: "ide"` and uses the same streamable HTTP client as other `mcp_servers[].url` entries. |
| **Bearer / auth** | Prefer `headers.Authorization` in the lockfile, or use `mcp_servers` with `bearer_token_file` for static tokens (see [`docs/goclaw/mcp-remote.md`](../goclaw/mcp-remote.md)). |
| **Post-tool notify** | `GOCLAW_IDE_NOTIFY_URL` must stay **loopback-only**. Payload shape is stable JSON: `{"tool","result_bytes","is_error"}`. Extensions may use this for progress UI without MCP. |
| **Future events** | Additional notification types should be versioned (e.g. `event` field) and remain **best-effort**; the REPL must not depend on the IDE replying. |
| **OAuth / remote IDE** | Out of scope until explicitly designed; same posture as D6 OAuth for MCP. |

**Reference implementation:** [`goclaw/internal/ide/discovery.go`](../../goclaw/internal/ide/discovery.go), [`goclaw/internal/ide/notify.go`](../../goclaw/internal/ide/notify.go), wiring in [`goclaw/internal/app/chat_wiring.go`](../../goclaw/internal/app/chat_wiring.go).

---

## 5. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: IDE local vs Bridge remote, discovery, transports, localhost threat, Go mapping, **D21** |
| 2026-04-07 | §6: `GOCLAW_IDE_NOTIFY_URL`, `internal/ide/notify.go`, scope vs full D21 |
| 2026-04-09 | §6: `ide_bridge_mcp`, `internal/ide/discovery.go`, lockfile JSON → MCP HTTP |
| 2026-04-09 | §7: goclaw ↔ extension contract (lockfile MCP, notify payload, future versioning) |
| 2026-04-12 | Translated Spanish sections 1–4 and changelog to English |
