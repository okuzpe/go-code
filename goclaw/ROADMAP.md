# goclaw — Functional Product Roadmap

Prioritized checklist toward a fully working, shippable coding agent.
Each tier must be stable before starting the next.

## What “daily driver” means here

- **Clear progress signals**: the UI should show “thinking / running tool / done” without dumping raw JSON.
- **Predictable safety**: tools have caps and permissions; failures are explicit.
- **Green means green**: verification should not silently hang; CI should enforce timeouts where possible.

---

## Tier 0 — Build + Tests Green (do this first)

- [x] Run `go build ./...` from `goclaw/` — confirm zero errors
- [x] Run `go test ./...` — confirm all tests pass (`-timeout 10m` in CI). **Note:** `internal/ui/chat` has **no** `_test.go` files today — full Bubble Tea / textarea init hung test runs on Windows/Git Bash; rely on **1a manual checks** for TUI until we add **pure-Go** tests (no full program init) or a gated build tag.
- [x] Run `go vet ./...` — confirm zero vet warnings (note: verified for changed packages; still run full `go vet ./...` as part of Tier 0)
- [x] Verify CI workflow (`.github/workflows/goclaw-ci.yml`) triggers correctly on push to `master`/`main` (repo-root workflow; `paths: goclaw/**`; matrix **ubuntu-latest** + **windows-latest**; `go test -race` **Ubuntu only**, plain `go test` on Windows)
- [ ] Check `go test -race ./...` passes on **your** machine (race needs CGO; **green on Linux CI**; local Windows needs a C toolchain / `CGO_ENABLED=1` if you want parity)

---

## Tier 1 — Core Quality (makes the tool actually usable today)

### 1a. TUI polish
- [ ] Test `chat.RunApp` end-to-end: verify the BubbleTea TUI launches, accepts input, and shows streaming output correctly (note: tool UI was redesigned to be Claude Code–style: no raw JSON, compact tool done lines, footer status while running)
- [ ] Verify approval modal (tool allow/deny) renders and responds to `y`/`n` keyboard input (note: preview now uses human-readable summary, not raw JSON)
- [ ] Verify `Ctrl+L` clears the viewport without crashing (note: now also clears pending tool queue)
- [ ] Verify `Ctrl+C` saves the session cleanly before exit
- [x] **Session id visible in the TUI** — footer status line appends `sess·…` (compact / rune-safe) via `internal/ui/footerline` + `footerline.Join` from `internal/ui/chat/chat.go` (truncates primary status on narrow widths). **Title bar** stays compact (`goclaw · provider · model · profile` only). Tests live in `internal/ui/footerline` (no Bubble Tea init).

### 1b. Non-TTY / readline path
- [x] Readline history file: `replHistoryFile` → `<UserConfigDir>/history` (`internal/app/repl_readline.go`); `MkdirAll` on config dir before `readline.NewEx` so the file can always be created; chzyer/readline persists on `Close`
- [ ] Verify readline history persists across restarts (manual: type a line, exit, restart — should appear on ↑)
- [ ] Verify `rl.Close()` goroutine doesn't race with the REPL loop on `ctx.Done()`
  - Notes (Issue #3/#7): `internal/app/repl_readline.go` keeps `sess` as a local pointer that slash commands may replace (e.g. `/new`), then **syncs back** to `rt.Sess` after each `HandleSlash` call. The signal goroutine only cancels the in-flight request (via `reqCancelFn`) and closes readline; it never mutates the session. This is intentionally structured so any apparent race is **benign**: once `RunStreaming` returns, no goroutine is still writing to the session before the REPL touches it again.

### 1c. Error messages — user-facing clarity
- [x] Ollama dial / timeout / DNS / TLS / generic dial errors: actionable hints in `wrapOllamaDialErr` (`internal/llm/ollama.go`); tests in `ollama_err_test.go`
- [x] Missing `ANTHROPIC_API_KEY` / `api_key`: clearer startup error in `PrepareChatRuntime` (env + user + project `settings.json` + `goclaw doctor`)
- [x] Unknown profile: dynamic list via `agents.ProfileListHint()` in `chat_wiring.go`, `/profile` slash usage, and `--profile` flag help (`internal/cli/root.go`)

### 1d. Session persistence
- [x] `/new` — old transcript is saved before switching (assertions in `TestHandleSlashNewAndSave`: load previous id, `Len()==1`, new session empty)
- [x] Resume / tool turns — `TestStoreRoundtripWithToolTurn` encodes assistant `tool_calls` + user `tool_results` + follow-up user text through JSONL (`internal/session/store_test.go`)
- [x] Session JSONL rotation — `Save` rotates when current file ≥ 256 KiB (`rotateAfterBytes`, max 3 numbered tails); `TestStoreListIDsIgnoresRotationFiles` ensures `sessions list` does not surface `.N.jsonl` shards
  - Notes (Issue #3/#7): session pointer replacement is done via `orchestrator.ReplaceSession`, while the REPL loop syncs the local `sess` pointer back into runtime (`rt.Sess`) after slash handling.

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
- [ ] Test: `tool_permissions` with unknown mode string → `ParseMode` returns error (check `PrepareChatRuntime` / `policy.ApplyConfigModes` in `internal/app/chat_wiring.go`)
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
- [x] Status line shows current profile name (now in compact title bar: `goclaw · provider · model · profile`)
- [x] Tool approval modal shows a truncated path/content preview, not raw JSON
- [ ] Streaming indicator shows elapsed seconds for long tool calls
- [x] Error messages are styled differently from system messages (red vs. grey)
- [x] Markdown rendering in assistant responses (uses `glamour`; wrap follows terminal width)

### 4b. REPL improvements
- [x] `Tab` completion for **top-level** slash commands (`slashcmd.ReadlinePrefixCompleter` wired in `runReadlineREPL`; list in `readline_tab.go` — extend when adding `/` roots)
- [ ] `/sessions` shows session creation timestamp, not just ID
- [ ] `/compact` shows before/after message counts in the confirmation line
- [ ] `/memory list` shows entry body preview (first 80 chars)

### 4c. Startup experience
- [ ] Startup banner shows workspace path (not just provider/model/profile)
- [ ] If Ollama is unreachable at startup, show warning but don't block REPL start
- [x] **`doctor` preflight** — `goclaw doctor` and slash `/doctor`: workspace, provider, paths, Ollama probe + hints, MCP server list vs connections, effective `tool_permissions` per registered tool (`internal/app/doctor.go`)
- [x] `--version` flag prints version (`cmd/goclaw/version.go` provides `var Version = "dev"` and can be set via ldflags)

---

## Tier 4b — Security Hardening (research findings)

> Items from Trail of Bits + Anthropic security research (2025). Address before v2.

- [x] **Argument injection via `find -exec`/`xargs`/`go test -exec`**: `rejectDangerousArgs` in `bash.go` blocks `-exec`/`-execdir` for `find`, validates the xargs command against the allowlist, and blocks `go test -exec`. Tests added to `bash_test.go`.
- [x] **`curl`/`wget` bypass SSRF**: `rejectSSRFInNetworkArgs` in `bash.go` parses URLs in curl/wget arguments and applies the same `checkHostResolvedIPs` check as `web_fetch`. Tests added.
- [x] **MCP `CallTool` per-request timeout**: already implemented — `adapter.go` wraps each `CallTool` in `context.WithTimeout(ctx, MCPToolCallTimeout=60s)`; `readLine` properly selects on `ctx.Done()`.
- [x] **MCP server-initiated requests**: JSON-RPC messages with both `method` and `id` set (server → client requests) receive a JSON-RPC error response (-32601); covered by `internal/mcp/session_test.go`.
- [ ] **MCP reconnect policy**: a crashed MCP subprocess after startup is undetected until the next tool call fails with an I/O error. Add a restart/reconnect attempt or surface the error clearly.
- [x] **Context budget is not model-aware**: replaced `defaultContextBudgetChars` with `anthropicContextTokens=200_000` / `ollamaContextTokens=32_000` per-provider constants. Configurable via `model_context_tokens` in settings.json for non-standard Ollama models.
- [x] **Tool-result clearing as first compaction stage**: `maybeCompact` now runs two phases — phase 1 clears `ToolResults[].Content` in old turns (replaces with `"[compacted]"`), phase 2 (`compactToTail`) only runs if phase 1 alone wasn't enough. Tests added to `estimate_test.go`.

## Tier 5 — Observability & CI (team-ready)

- [x] Add `go test -coverprofile=coverage.out ./...` step to CI and set a minimum threshold (70%)
- [x] Add `golangci-lint run` step to CI (`.golangci.yml` config added)
- [x] Log LLM request duration and tool count at `slog.Debug` level in the orchestrator
- [x] Log MCP tool call round-trip latency in `mcp/adapter.go`
- [ ] Add `--version` flag output to CI smoke test

---

## Tier 6 — Post-MVP: Coordinator + Custom Agents (v2+)

> Only start after Tier 0–3 are solid. These are architectural changes.

- [x] **D16 Coordinator** — implemented `internal/coordinator` package
  - [x] `WorkerNotification` JSON type (`profile`, `status`, `summary`, `result`)
  - [x] Coordinator profile with delegation-only tool allowlist (`spawn_agent`, `todo_write`)
  - [x] `spawn_agent` coordinator tool (workers isolated via `session.New()`; nesting prevented)
  - [x] Timeout + context cancellation via `context.WithTimeout` (default 120 s, max 600 s)
  - [x] Mock-server tests for spawn_agent success, failure, nesting rejection, invalid input
  - [ ] `stop_task` tool (post-MVP — not yet implemented)
- [x] **D17 YOLO Classifier** — rule-based risk scorer (0–100); `yolo_threshold: -1` default (off); auto-approves reads at threshold 0; see `internal/permissions/risk.go`
- [x] **D19 Custom agents** — load `~/.goclaw/agents/*.md` and `.goclaw/agents/*.md` as profiles at startup
  - [x] YAML frontmatter parser for `name`, `model`, `tool_allowlist`, `read_only`, `system_prompt`; body appended to system prompt
  - [x] Hot-reload on `/profile` command (re-scans agent dirs without restart)
  - [x] Project overrides user; user overrides built-in (same layering as config)
  - [x] Custom profiles available to `spawn_agent` workers via `WithProfiles`

---

---

## Tier 6b — Agent Power Features (v2)

> Features that transform goclaw from a capable CLI into a daily-driver coding agent.

- [x] **Shell power** — `script` tool for multi-line shell scripts (pipes, `&&`, redirections); opt-in via `allow_script: true` in settings.json; reuses bash timeout; separate tool name so permissions can differ (`bash: allow, script: ask`)
- [x] **D17 YOLO Classifier** — see Tier 6 above
- [x] **D19 Custom agents** — see Tier 6 above
- [x] **Parallel read-tool execution** — concurrent `sync.WaitGroup` fan-out when all tools in a turn auto-approve; deterministic result ordering via pre-allocated index slots; sequential fallback for any interactive-approval turn
- [x] **LLM-driven compaction** — LLM summarizes removed context instead of heuristic placeholder; opt-in via `llm_compaction: true`; falls back to heuristic on LLM error

---

## Tier 7 — Infrastructure (optional quality-of-life)

- [x] `Makefile` with targets: `build`, `test`, `lint`, `race`, `cover`, `clean`
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
| `doctor` / `/doctor` health report (Ollama, MCP, permissions) | Done |
| All slash commands (`/help` … `/apply-plan`) | Done |
| External hooks (subprocess + HTTP) + project hooks file | Done |
| MCP stdio client (multi-server) | Done |
| IDE localhost notifier (`GOCLAW_IDE_NOTIFY_URL`) | Done |
| CI: `go vet` + tests on Linux and Windows; `-race` on Linux only | Done |
| CI: `golangci-lint` + 70% coverage threshold | Done |
| 6 built-in agent profiles | Done |
| Custom agent profiles (`~/.goclaw/agents/*.md`, `.goclaw/agents/*.md`) | Done |
| `script` tool (multi-line shell, opt-in via `allow_script: true`) | Done |
| YOLO risk classifier (`yolo_threshold` in settings, default -1/off) | Done |
| Parallel read-tool execution (auto-approve turns only) | Done |
| LLM-driven compaction (`llm_compaction: true`, opt-in) | Done |
| `slog.Debug` timing for LLM stream and MCP tool calls | Done |
