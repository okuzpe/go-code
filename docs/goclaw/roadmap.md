# goclaw — Functional Product Roadmap

Prioritized checklist toward a fully working, shippable coding agent.
Each tier must be stable before starting the next.

## What “daily driver” means here

- **Clear progress signals**: the UI should show “thinking / running tool / done” without dumping raw JSON.
- **Predictable safety**: tools have caps and permissions; failures are explicit.
- **Green means green**: verification should not silently hang; CI should enforce timeouts where possible.

**Where this file lives:** Product checklist **[roadmap.md](roadmap.md)** under **`docs/goclaw/`**. Project entry: **[`README.md`](../../goclaw/README.md)**; master index: **[`docs-map.md`](../docs-map.md)**. It does not duplicate **`docs/reference/`** specs.

**MVP scope (shippable CLI):** Treat **Tiers 0–8** below as the **MVP / daily-driver checklist**. Every `- [x]` item in those tiers is **in scope for MVP closure**; anything still open must be either **unchecked only under [Future transport and scale](#future-transport-and-scale) or [Future — UI localization](#future--ui-localization)** (explicitly *not* required for the current CLI), or called out as **Partial** in [`docs-map.md`](../docs-map.md) (IDE parity, MCP OAuth/WS, remote plugin marketplace, Team/Swarm peer topology vs shipped `coordinator`). When all Tier 0–8 boxes are checked, the product is **MVP-complete per this document**; remaining work is **post-MVP** unless you intentionally narrow MVP to an older tier range.

---

## Post-MVP waves (ordering)

Use this table to pick the **next** engineering theme after MVP closure. **Default first slice:** **Wave A** (better IDE/editor story on top of lockfile MCP + `GOCLAW_IDE_NOTIFY_URL`) when the pain is editor integration; switch to **Wave B** if enterprise MCP (OAuth/WS, non-loopback policy) is the blocker.

| Wave | Focus | Primary docs | First concrete steps |
|------|--------|----------------|----------------------|
| **A** | IDE extension / editor MCP parity | [ide-bridge.md](../reference/ide-bridge.md), [docs-map.md](../docs-map.md) IDE row | Ship or document one reference extension flow; tighten discovery + failure modes; align §6–§7 with `internal/ide`. |
| **B** | MCP enterprise | [mcp-remote.md](mcp-remote.md), [mcp.md](../reference/mcp.md) | OAuth or token refresh story; optional WS transport; policy for non-loopback URLs with `mcp_allow_remote_urls`. |
| **C** | Multi-client / platform | [Future transport and scale](#future-transport-and-scale) | Long-lived gateway; channel adapters; headless one-shot runs with cancelable `context`. |
| **D** | Static UI i18n | [Future — UI localization](#future--ui-localization), [i18n.md](i18n.md) | `ui_locale` / env; `internal/locale` catalogs; keep docs in sync. |

---

## Tier 0 — Build + Tests Green (do this first)

- [x] Run `go build ./...` from `goclaw/` — confirm zero errors
- [x] Run `go test ./...` — confirm all tests pass (`-timeout 10m` in CI). **Note:** `internal/ui/chat` intentionally has **no** `_test.go` files — on some Windows environments `go test` for that package can hang while linking the test binary (Bubble Tea / glamour stack). **Pure** UI string logic is covered in **`internal/text`** (`TruncateRunes`, formerly in `chat.go`). TUI flows remain in [`manual-tui-checklist.md`](./manual-tui-checklist.md).
- [x] Run `go vet ./...` — confirm zero vet warnings (note: verified for changed packages; still run full `go vet ./...` as part of Tier 0)
- [x] Verify CI workflow (`.github/workflows/goclaw-ci.yml`) triggers correctly on push to `master`/`main` (repo-root workflow; `paths: goclaw/**`; matrix **ubuntu-latest** + **windows-latest**; `go test -race` **Ubuntu only**, plain `go test` on Windows)
- [x] Check `go test -race ./...` — **green on Linux CI**; local Windows optional (documented in `.cursor/rules/workflow.mdc`: CGO + C toolchain for parity)

---

## Tier 1 — Core Quality (makes the tool actually usable today)

### 1a. TUI polish

- [x] Manual checklist: [`manual-tui-checklist.md`](./manual-tui-checklist.md) — RunApp, stream, modal `y`/`n`, `Ctrl+L`, `Ctrl+C` + save
- [x] **Session id visible in the TUI** — footer uses a two-line layout in `internal/ui/chat/chat.go`: primary row for spinner / tool / “Responding…”, second row for `Theme.FooterHint()` plus `sess·…` via `footerline.HintsWithSession` (wraps session to the next line when narrow). **Title bar** stays compact (`goclaw · provider · model · profile` only). Tests live in `internal/ui/footerline` (no Bubble Tea init).

### 1b. Non-TTY / readline path

- [x] Readline history file: `replHistoryFile` → `<UserConfigDir>/history` (`internal/app/repl_readline.go`); `MkdirAll` on config dir before `readline.NewEx` so the file can always be created; chzyer/readline persists on `Close`
- [x] History path test (`repl_readline_test.go`); persistence: manual step in [`manual-tui-checklist.md`](./manual-tui-checklist.md)
- [x] `rl.Close()` / signal goroutine: documented benign pattern in `repl_readline.go` (Issue #3/#7); no shared `sess` mutation from signal path
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

- [x] Test: `old_string` matching 0 times → clear error with match count (`edit_file_test.go`)
- [x] Test: 2+ matches without `replace_all: true` → clear error
- [x] Test: Windows line endings (`\r\n`) round-trip
- [x] Test: read-only file (Unix) → graceful error

### 2b. `write_file` edge cases

- [x] Test: parent directory missing → clear error (`write_file_test.go`)
- [x] Test: content exactly `MaxWriteFileBytes` succeeds
- [x] Test: 1 byte over cap → size error

### 2c. `bash` tool robustness

- [x] Timeout path: `sleep 60` killed under short timeout (non-Windows; `bash_test.go`)
- [x] `bash_timeout_sec` in settings → loader + `BashTimeoutSeconds()` (`config/loader_test.go`)
- [x] Single-quoted string containing `&&` allowed (`bash_test.go`)
- [x] `git.exe` first token normalizes like `git` on Windows (`allowlistBinaryName` in `bash.go`)

### 2d. `web_fetch` / `web_search`

- [x] SSRF on redirect targets: `validateRedirectURL` / metadata / RFC1918 (`ssrf_test.go`; same checks as `CheckRedirect`)
- [x] `web_search` empty `Results` / thin DDG JSON → fallback message with duckduckgo link (`web_search_test.go`)
- [x] Redirect to private IP blocked (validator tests; no live HTTP redirect needed)

### 2e. `todo_write` tool

- [x] Session task list in system prompt (`build_request_test.go` `TestBuildRequestInjectsTodoBlock`)
- [x] `todos.Store.Clear()` on `/new` via `ReplaceSession` (`orchestrator_test.go`)
- [x] Test: 51 items → rejected (`todo_write_test.go`)
- [x] Test: content > 500 runes → rejected

---

## Tier 3 — Configuration + Hooks (production settings)

### 3a. Config loader

- [x] Project `settings.json` overrides user for same key (`loader_test.go`)
- [x] `settings.local.json` precedence (user + project) (`loader_test.go`)
- [x] Invalid JSON reports file path (`loader_test.go`)
- [x] `tool_permissions` unknown mode → `ParseMode` / `ApplyConfigModes` error (`permissions_test.go`)
- [x] `bash_timeout_sec: 0` falls back via `BashTimeoutSeconds()` (`loader_test.go`)
- [x] `bash_timeout_sec: 3601` clamped to 3600 (`loader_test.go`)

### 3b. External hooks

- [x] `OnCommand` + `PreToolUse`: JSON on stdin includes `tool_name` / `tool_input` (`hooks_test.go`)
- [x] `OnHTTP` POST body matches event schema (`hooks_test.go`, transport to `example.com`)
- [x] Project `hooks.json` only when `trusted_workspace: true` (`chat_wiring_test.go`)
- [x] `PreToolUse` external hook exit 1 blocks (`hooks_test.go`); exit 2 message already covered

### 3c. MCP integration

- [x] Failed MCP server does not abort startup (`chat_wiring_test.go`)
- [x] `mcp__<server_id>__<tool_name>` after `RegisterSessionTools` (`adapter_register_test.go`)
- [x] Read-only profile blocks `mcp__` in `executeTool` (`tool_exec_test.go`); specs strip already in `build_request_test.go`
- [x] Piped mock `tools/list` registers tools (`adapter_register_test.go`, `session_test.go`)
- [x] MCP tool call round-trip into live session history (`TestOrchestratorMCPRoundTripRecordsToolResultInSession`; piped `CallTool` in `session_test.go`)

---

## Tier 4 — UX & Polish (daily driver quality)

### 4a. TUI visual improvements

- [x] Status line shows current profile name (now in compact title bar: `goclaw · provider · model · profile`)
- [x] Tool approval modal shows a truncated path/content preview, not raw JSON
- [x] Footer shows elapsed seconds while a tool is running (`internal/ui/chat/chat.go` + tick)
- [x] Error messages are styled differently from system messages (red vs. grey)
- [x] Markdown rendering in assistant responses (uses `glamour`; wrap follows terminal width)

### 4b. REPL improvements

- [x] `Tab` completion for **top-level** slash commands (`slashcmd.ReadlinePrefixCompleter` wired in `runReadlineREPL`; list in `readline_tab.go` — extend when adding `/` roots)
- [x] `/sessions` lists id + file mtime (RFC3339) via `Store.ListSessionEntries`
- [x] `/compact` shows message counts before → after
- [x] `/memory list` body preview (80 runes)

### 4c. Startup experience

- [x] Readline / non-TTY startup: `printStartupBanner` shows workspace (TTY: work line in banner; non-TTY: `Workspace:` line). Default TUI skips the banner (welcome panel instead).
- [x] Ollama unreachable: `slog.Warn` after `PrepareChatRuntime`, non-blocking (`ollama_probe.go`)
- [x] **`doctor` preflight** — `goclaw doctor` and slash `/doctor`: workspace, provider, paths, Ollama probe + hints, MCP server list vs connections, effective `tool_permissions` per registered tool (`internal/app/doctor.go`)
- [x] `--version` flag prints version (`cmd/goclaw/version.go` provides `var Version = "dev"` and can be set via ldflags)

---

## Tier 4b — Security Hardening (research findings)

> Items from Trail of Bits + Anthropic security research (2025). Address before v2.

- [x] **Argument injection via `find -exec`/`xargs`/`go test -exec`**: `rejectDangerousArgs` in `bash.go` blocks `-exec`/`-execdir` for `find`, validates the xargs command against the allowlist, and blocks `go test -exec`. Tests added to `bash_test.go`.
- [x] **`curl`/`wget` bypass SSRF**: `rejectSSRFInNetworkArgs` in `bash.go` parses URLs in curl/wget arguments and applies the same `checkHostResolvedIPs` check as `web_fetch`. Tests added.
- [x] **MCP `CallTool` per-request timeout**: already implemented — `adapter.go` wraps each `CallTool` in `context.WithTimeout(ctx, MCPToolCallTimeout=60s)`; `readLine` properly selects on `ctx.Done()`.
- [x] **MCP server-initiated requests**: JSON-RPC messages with both `method` and `id` set (server → client requests) receive a JSON-RPC error response (-32601); covered by `internal/mcp/session_test.go`.
- [x] **MCP dead connection**: `tools/call` surfaces EOF/closed connection explicitly (`session.go`); **one-shot reconnect** on recoverable transport errors via `mcp.ResilientConn` in `chat_wiring.go` (`resilient.go`, `resilient_test.go`)
- [x] **Context budget is not model-aware**: replaced `defaultContextBudgetChars` with `anthropicContextTokens=200_000` / `ollamaContextTokens=32_000` per-provider constants. Configurable via `model_context_tokens` in settings.json for non-standard Ollama models.
- [x] **Tool-result clearing as first compaction stage**: `maybeCompact` now runs two phases — phase 1 clears `ToolResults[].Content` in old turns (replaces with `"[compacted]"`), phase 2 (`compactToTail`) only runs if phase 1 alone wasn't enough. Tests added to `estimate_test.go`.

## Tier 5 — Observability & CI (team-ready)

- [x] Add `go test -coverpkg=./... -coverprofile=coverage.out ./...` step to CI and set a minimum threshold (61%). Note: `internal/ui/chat` (BubbleTea fullscreen) and REPL entry-point functions are not unit-testable; ~61–63% is the honest ceiling for per-package + cross-package measurement without integration tests.
- [x] Add `golangci-lint run` step to CI (`.golangci.yml` config added)
- [x] Log LLM request duration and tool count at `slog.Debug` level in the orchestrator
- [x] Log MCP tool call round-trip latency in `mcp/adapter.go`
- [x] `--version` in CI (`go run ./cmd/goclaw --version` in `goclaw-ci.yml`)

---

## Tier 6 — Post-MVP: Coordinator + Custom Agents (v2+)

> Only start after Tier 0–3 are solid. These are architectural changes.

- [x] **D16 Coordinator** — implemented `internal/coordinator` package
  - [x] `WorkerNotification` JSON type (`profile`, `status`, `summary`, `result`)
  - [x] Coordinator profile allowlist (`spawn_agent`, `stop_task`, `todo_write`)
  - [x] `spawn_agent` coordinator tool (workers isolated via `session.New()`; nesting prevented)
  - [x] Timeout + context cancellation via `context.WithTimeout` (default 120 s, max 600 s)
  - [x] Mock-server tests for spawn_agent success, failure, nesting rejection, invalid input
  - [x] `stop_task` tool — cancel in-flight worker via `task_id` from `spawn_agent` JSON
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
- [x] `goreleaser` config (`.goreleaser.yaml`; run `goreleaser release` locally or in CI)
- [x] `docs/goclaw/changelog.md` (Keep a Changelog–style; tag-driven notes via goreleaser)
- [x] Remote MCP **Streamable HTTP** client (`internal/mcp/http.go`): `mcp_servers[].url`, optional `headers`; loopback-only unless `mcp_allow_remote_urls` in settings (see CLAUDE.md D6)
- [x] IDE bridge **lockfile discovery** (`ide_bridge_mcp` + `~/.goclaw/ide/*.json` → synthetic server `id=ide`); full editor MCP parity still extension-dependent (see [ide-bridge.md](../reference/ide-bridge.md))
- [x] LLM-driven compaction — opt-in via `llm_compaction: true` (see Tier 6b); heuristic fallback remains

---

## Quick reference — what is already done

| Area                                                                   | Status |
| ---------------------------------------------------------------------- | ------ |
| Core agent loop (32 iter / 64 tools)                                   | Done   |
| Ollama + Anthropic clients with retry                                  | Done   |
| Session JSONL store + resume                                           | Done   |
| `read_file`, `glob`, `grep`, `bash`, `write_file`, `edit_file`         | Done   |
| `web_fetch` (SSRF guards), `web_search` (DDG)                          | Done   |
| `todo_write` + session task list in prompt                             | Done   |
| Permissions (ask/allow/deny) + tool approver                           | Done   |
| Memory filesystem (`~/.goclaw/memory/`)                                | Done   |
| Context compaction (threshold + force)                                 | Done   |
| Readline REPL (history, arrow keys)                                    | Done   |
| BubbleTea TUI (TTY mode)                                               | Done   |
| Cobra CLI + `sessions list` subcommand                                 | Done   |
| `doctor` / `/doctor` health report (Ollama, MCP, permissions)          | Done   |
| All slash commands (`/help` … `/apply-plan`)                           | Done   |
| External hooks (subprocess + HTTP) + project hooks file                | Done   |
| MCP stdio + Streamable HTTP client (multi-server)                      | Done   |
| IDE lockfile MCP discovery (`ide_bridge_mcp`, `~/.goclaw/ide/*.json`)  | Done   |
| IDE localhost notifier (`GOCLAW_IDE_NOTIFY_URL`)                       | Done   |
| CI: `go vet` + tests on Linux and Windows; `-race` on Linux only       | Done   |
| CI: `golangci-lint` + 61% coverage threshold (`-coverpkg=./...`)       | Done   |
| 7 built-in agent profiles (incl. `coordinator`)                        | Done   |
| Custom agent profiles (`~/.goclaw/agents/*.md`, `.goclaw/agents/*.md`) | Done   |
| `script` tool (multi-line shell, opt-in via `allow_script: true`)      | Done   |
| YOLO risk classifier (`yolo_threshold` in settings, default -1/off)    | Done   |
| Parallel read-tool execution (auto-approve turns only)                 | Done   |
| LLM-driven compaction (`llm_compaction: true`, opt-in)                 | Done   |
| `slog.Debug` timing for LLM stream and MCP tool calls                  | Done   |

---

## Tier 8 — V3+ slice (2026-04)

First-pass implementation of roadmap items that were still “post-MVP” on paper; **not** full enterprise parity (no MCP OAuth/WS here).

- [x] **MCP remote token file** — `mcp_servers[].bearer_token_file` merged into HTTP `Authorization` when headers omit it ([`internal/app/chat_wiring.go`](../../goclaw/internal/app/chat_wiring.go)); notes in [`mcp-remote.md`](./mcp-remote.md)
- [x] **Local plugins (hooks MVP)** — [`internal/plugin`](../../goclaw/internal/plugin): `goclaw-plugin.json`, `plugin_dirs` / `plugin_allow` / `plugin_deny` in settings, `--plugin-dir` flag
- [x] **Memory auto-capture (opt-in)** — `memory_auto_extract` → short project line after successful `write_file` / `edit_file` ([`internal/memory/autocapture.go`](../../goclaw/internal/memory/autocapture.go))
- [x] **Runtime skills** — [`internal/skills`](../../goclaw/internal/skills) + orchestrator `WithSkillsSnippet` (`.goclaw/skills`, `.claude/skills`, user mirrors)
- [x] **Swarm disk hub** — [`internal/swarm`](../../goclaw/internal/swarm) + [`swarm.md`](./swarm.md) (vs coordinator)
- [x] **IDE extension contract** — [ide-bridge.md](../reference/ide-bridge.md) §7
- [x] **Docs sync** — [`CLAUDE.md`](../../goclaw/CLAUDE.md), [`docs-map.md`](../docs-map.md), this tier

---

## Future transport and scale

Optional product directions **not** required for the current CLI. Captures design ideas from comparing goclaw to larger multi-channel agent products (see [philosophy.md — Lessons from wider agent stacks](philosophy.md#lessons-from-wider-agent-stacks)).

- [ ] **Long-lived gateway process** — single control plane for sessions + routing when multiple clients attach (contrast with today’s one-process REPL/TUI).
- [ ] **Channel adapters** — Discord/Telegram/Slack-style transports behind a **queue + per-channel allowlist**, without pushing transport code into the orchestrator core.
- [ ] **Scheduled / isolated runs** — cron-style or headless “run this prompt once” using the same orchestrator with a **cancelable** `context` (no new agent model required).

---

## Future — UI localization (i18n)

LLM replies already follow the user's language via [`internal/orchestrator/base_system_prompt.md`](../../goclaw/internal/orchestrator/base_system_prompt.md) rule 8. Static UI strings remain English until below is done.

- [ ] **`ui_locale` / env** — select language for banners, `/capabilities`, onboarding, fixed errors
- [ ] **Message catalogs** — `internal/locale` (or similar) with fallback to English
- [ ] **Docs** — keep [`i18n.md`](./i18n.md) updated as behavior changes
