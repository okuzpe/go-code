# goclaw — Functional Product Roadmap

Prioritized checklist toward a fully working, shippable coding agent.
Each tier must be stable before starting the next.

---

## Tier 0 — Build + Tests Green (do this first)

- [ ] Run `go build ./...` from `goclaw/` — confirm zero errors
- [ ] Run `go test ./...` — confirm all tests pass (including new TUI/chat package)
- [ ] Run `go vet ./...` — confirm zero vet warnings
- [ ] Verify CI workflow (`.github/workflows/goclaw-ci.yml`) triggers correctly on push to `master`/`main`
- [ ] Check `go test -race ./...` passes (race detector; may require CGO/Linux CI)

---

## Tier 1 — Core Quality (makes the tool actually usable today)

### 1a. TUI polish
- [ ] Test `chat.RunApp` end-to-end: verify the BubbleTea TUI launches, accepts input, and shows streaming output correctly
- [ ] Verify approval modal (tool allow/deny) renders and responds to `y`/`n` keyboard input
- [ ] Verify `Ctrl+L` clears the viewport without crashing
- [ ] Verify `Ctrl+C` saves the session cleanly before exit
- [ ] Verify session ID appears in the window title bar

### 1b. Non-TTY / readline path
- [ ] Verify readline history persists across restarts (`~/.goclaw/history`)
- [ ] Verify `rl.Close()` goroutine doesn't race with the REPL loop on `ctx.Done()`

### 1c. Error messages — user-facing clarity
- [ ] Ollama dial error shows "Is Ollama running?" hint (check `wrapOllamaDialErr` in `ollama.go`)
- [ ] Missing `ANTHROPIC_API_KEY` shows a clear message (done in `chat_run.go` line 108 — verify output)
- [ ] Unknown `--profile` shows the valid list (done in `chat_run.go` line 100 — verify message format)

### 1d. Session persistence
- [ ] `/new` — verify old session is saved to disk before the new one starts
- [ ] `--session <id>` — verify resumed session merges tool-result turns correctly (no deserialization gaps)
- [ ] Session JSONL rotation: verify files don't grow unbounded (check `session/store.go`)

---

## Tier 2 — Tool Robustness (quality for coding tasks)

### 2a. `edit_file` edge cases
- [ ] Test: `edit_file` with `old_string` matching 0 times → clear error with match count
- [ ] Test: `edit_file` with `old_string` matching 2+ times without `replace_all: true` → clear error
- [ ] Test: `edit_file` on a file with Windows line endings (`\r\n`) — does it round-trip cleanly?
- [ ] Test: `edit_file` on a read-only file (permissions) → graceful error, not panic

### 2b. `write_file` edge cases
- [ ] Test: `write_file` with a path whose parent directory does not exist → clear error (no crash)
- [ ] Test: `write_file` with content at exactly `MaxWriteFileBytes` (1 MiB) — confirm it succeeds
- [ ] Test: `write_file` with content 1 byte over cap → rejected with size error

### 2c. `bash` tool robustness
- [ ] Verify timeout path: a `sleep 60` command is killed after `BashTimeoutSec` (30s default)
- [ ] Verify `bash_timeout_sec` in `settings.json` flows from config loader → `NewBashWithTimeout`
- [ ] Test `rejectShellMetacharacters` with a single-quoted string containing `&&` — confirm allowed
- [ ] Verify `git` alias binaries (e.g. `git.exe` on Windows) are recognised by `validateAllowlistedCommand`

### 2d. `web_fetch` / `web_search`
- [ ] Verify SSRF guards block `169.254.169.254` (AWS metadata) after redirects
- [ ] Verify `web_search` DuckDuckGo fallback message when results array is empty
- [ ] Add test: `web_fetch` with a redirect to a private IP → blocked

### 2e. `todo_write` tool
- [ ] Verify session task list appears in system prompt (orchestrator injects via `WithTodoStore`)
- [ ] Verify `todos.Store.Clear()` is called on `/new` (confirm `ReplaceSession` calls `todoStore.Clear()`)
- [ ] Add test: 51 items → rejected with item count error
- [ ] Add test: item content > 500 runes → rejected

---

## Tier 3 — Configuration + Hooks (production settings)

### 3a. Config loader
- [ ] Test: project `settings.json` overrides user `settings.json` for the same key
- [ ] Test: `settings.local.json` takes precedence over `settings.json` (both user and project)
- [ ] Test: invalid JSON in any settings file → clear error with which file failed
- [ ] Test: `tool_permissions` with unknown mode string → `ParseMode` returns error (check `chat_run.go` wiring)
- [ ] Verify `bash_timeout_sec: 0` falls back to default 30s (not zero timeout)
- [ ] Verify `bash_timeout_sec: 3601` is clamped to 3600 (check `normalizeBashTimeoutSec`)

### 3b. External hooks
- [ ] Test: `OnCommand` hook fires for `PreToolUse` and receives correct `tool_name` + `input` in env
- [ ] Test: `OnHTTP` hook fires with JSON body matching the event schema
- [ ] Test: project hooks file (`.goclaw/hooks.json`) is loaded only when `trusted_workspace: true`
- [ ] Test: a blocking `PreToolUse` hook (`exit 1`) prevents tool execution and returns `IsError=true`

### 3c. MCP integration
- [ ] Verify MCP server start failure is isolated (one bad server does not abort startup)
- [ ] Verify `mcp__<server_id>__<tool_name>` naming convention in `RegisterSessionTools`
- [ ] Verify `ReadOnly` profile strips all `mcp__` tools from spec list AND blocks in `executeTool`
- [ ] Add integration test: mock MCP server (piped) responds to `tools/list` → tool appears in registry
- [ ] Add integration test: MCP tool call round-trip → result injected into session

---

## Tier 4 — UX & Polish (daily driver quality)

### 4a. TUI visual improvements
- [ ] Status line shows current profile name (already in title bar — consider footer too)
- [ ] Tool approval modal shows a truncated path/content preview, not raw JSON
- [ ] Streaming indicator shows elapsed seconds for long tool calls
- [ ] Error messages are styled differently from system messages (red vs. grey)
- [ ] Markdown rendering in assistant responses (consider `glamour` or simple code block highlight)

### 4b. REPL improvements
- [ ] `Tab` completion for slash commands (readline `AutoComplete` config)
- [ ] `/sessions` shows session creation timestamp, not just ID
- [ ] `/compact` shows before/after message counts in the confirmation line
- [ ] `/memory list` shows entry body preview (first 80 chars)

### 4c. Startup experience
- [ ] Startup banner shows workspace path (not just provider/model/profile)
- [ ] If Ollama is unreachable at startup, show warning but don't block REPL start
- [ ] `--version` flag prints version (add `cmd/goclaw/version.go` with `var Version = "dev"`)

---

## Tier 4b — Security Hardening (research findings)

> Items from Trail of Bits + Anthropic security research (2025). Address before v2.

- [x] **Argument injection via `find -exec`/`xargs`/`go test -exec`**: `rejectDangerousArgs` in `bash.go` blocks `-exec`/`-execdir` for `find`, validates the xargs command against the allowlist, and blocks `go test -exec`. Tests added to `bash_test.go`.
- [x] **`curl`/`wget` bypass SSRF**: `rejectSSRFInNetworkArgs` in `bash.go` parses URLs in curl/wget arguments and applies the same `checkHostResolvedIPs` check as `web_fetch`. Tests added.
- [x] **MCP `CallTool` per-request timeout**: already implemented — `adapter.go` wraps each `CallTool` in `context.WithTimeout(ctx, MCPToolCallTimeout=60s)`; `readLine` properly selects on `ctx.Done()`.
- [ ] **MCP server-initiated requests**: JSON-RPC messages with both `method` and `id` set (server → client requests) are currently silently dropped. The MCP spec requires responding with a JSON-RPC error for unknown methods — otherwise the server may consider the connection broken.
- [ ] **MCP reconnect policy**: a crashed MCP subprocess after startup is undetected until the next tool call fails with an I/O error. Add a restart/reconnect attempt or surface the error clearly.
- [x] **Context budget is not model-aware**: replaced `defaultContextBudgetChars` with `anthropicContextTokens=200_000` / `ollamaContextTokens=32_000` per-provider constants. Configurable via `model_context_tokens` in settings.json for non-standard Ollama models.
- [x] **Tool-result clearing as first compaction stage**: `maybeCompact` now runs two phases — phase 1 clears `ToolResults[].Content` in old turns (replaces with `"[compacted]"`), phase 2 (`compactToTail`) only runs if phase 1 alone wasn't enough. Tests added to `estimate_test.go`.

## Tier 5 — Observability & CI (team-ready)

- [ ] Add `go test -coverprofile=coverage.out ./...` step to CI and set a minimum threshold (e.g. 70%)
- [ ] Add `golangci-lint run` step to CI (use `.golangci.yml` config)
- [ ] Log LLM request duration and token estimate at `slog.Debug` level in the orchestrator
- [ ] Log MCP tool call round-trip latency in `mcp/adapter.go`
- [ ] Add `--version` flag output to CI smoke test

---

## Tier 6 — Post-MVP: Coordinator + Custom Agents (v2+)

> Only start after Tier 0–3 are solid. These are architectural changes.

- [ ] **D16 Coordinator** — implement `internal/coordinator` package per `docs/D16_COORDINATOR_SKETCH.md`
  - [ ] `WorkerNotification` JSON type
  - [ ] Coordinator profile with delegation-only tool allowlist
  - [ ] `spawn_worker` / `stop_task` coordinator tools
  - [ ] Task ID registry + cancellation via context
  - [ ] Mock-server tests for multi-turn coordinator + fake worker responses
- [ ] **D17 YOLO Classifier** — auto-approve low-risk tools without user prompt (post-coordinator)
- [ ] **D19 Custom agents** — load `.goclaw/agents/*.md` as profiles at startup
  - [ ] YAML frontmatter parser for `name`, `model`, `tool_allowlist`, `read_only`, `system_prompt`
  - [ ] Hot-reload on `/profile` command (re-scan `.goclaw/agents/` directory)

---

## Tier 7 — Infrastructure (optional quality-of-life)

- [ ] `Makefile` with targets: `build`, `test`, `lint`, `race`, `clean`
- [ ] `goreleaser` config for cross-platform binaries (macOS/Linux/Windows)
- [ ] `CHANGELOG.md` with conventional commits tagging
- [ ] Remote MCP transport (SSE/HTTP) — only after security + transport design review (see CLAUDE.md D6)
- [ ] Full IDE bridge via lockfile MCP client (see `IDE_BRIDGE.md` reference)
- [ ] LLM-written compaction (replace heuristic summary with a real summarization call)

---

## Quick reference — what is already done

| Area | Status |
|------|--------|
| Core agent loop (32 iter / 64 tools) | Done |
| Ollama + Anthropic clients with retry | Done |
| Session JSONL store + resume | Done |
| `read_file`, `glob`, `grep`, `bash`, `write_file`, `edit_file` | Done |
| `web_fetch` (SSRF guards), `web_search` (DDG) | Done |
| `todo_write` + session task list in prompt | Done |
| Permissions (ask/allow/deny) + tool approver | Done |
| Memory filesystem (`~/.goclaw/memory/`) | Done |
| Context compaction (threshold + force) | Done |
| Readline REPL (history, arrow keys) | Done |
| BubbleTea TUI (TTY mode) | Done |
| Cobra CLI + `sessions list` subcommand | Done |
| All slash commands (`/help` … `/apply-plan`) | Done |
| External hooks (subprocess + HTTP) + project hooks file | Done |
| MCP stdio client (multi-server) | Done |
| IDE localhost notifier (`GOCLAW_IDE_NOTIFY_URL`) | Done |
| CI workflow with race detector | Done |
| 6 built-in agent profiles | Done |
