# GoClaw — Project Rules for AI Agents

## Identity
- **Project**: `goclaw` — Go CLI agent, equivalent to Claude Code, focused on local models via Ollama
- **Go module**: `github.com/okuzpe/goclaw`
- **Go version**: 1.26
- **Default provider**: local Ollama (`qwen2.5-coder:14b`)
- **Alternative provider**: Anthropic API (requires `ANTHROPIC_API_KEY`)
- **Repo root**: clone path + `/goclaw` (module root for `go build` / `go test`)

**Workspace note:** If the parent folder also contains `claw-code/`, treat it as **reference material only**. It is not part of this module, not covered by this roadmap, and must not be modified when implementing goclaw. All product code, tests, and phase work live under `goclaw/`.

---

## Language Rule — STRICT

**All code, comments, identifiers, commit messages, PR descriptions, and documentation must be written in English.**

```go
// CORRECT
// Execute runs the tool with the given JSON input and returns the result.
func (t *ReadFileTool) Execute(ctx context.Context, input string) (Result, error) {

// WRONG — Spanish comment
// Ejecuta el tool con el input JSON dado y devuelve el resultado.
```

**Check (optional):** search Markdown and rules for stray non-English copy:

```bash
# Run from the goclaw module root (this directory)
rg '[áéíóúñüÁÉÍÓÚÑÜ¿¡]' --glob '*.md' --glob '*.mdc' .
```

Expect hits only in (a) wrong-comment examples below, (b) `audit` skill regex that detects Spanish in `//` comments, or (c) similar deliberate patterns.

This applies to:
- Source code comments (`//` and `/* */`)
- Variable, function, type, and package names
- Error messages returned to the model
- Log messages (`slog.Info`, `slog.Error`)
- Test names and descriptions
- Git commit messages
- Markdown documentation inside the repo

---

## Naming — full words (no lazy abbreviations)

**Identifiers must use complete words.** Do not use *lazy* abbreviations: shortened words with no stable Go meaning, only to type less.

- **Definition:** If expanding the name sounds like half a word, spell it out or use a domain term (`payload`, `sessionID`, `rendered`).
- **Avoid:** `fun` / `fn`; `re` / `res` for “result”; generic `str`; vague `num` / `cnt` / `tmp`; opaque `v` / `x` in non-trivial logic.
- **Keep:** Idiomatic shorts: `ctx`, `err`, `ok`, `i` / `j` / `k`, `t *testing.T`, `tb *testing.B`, `r` / `w` on HTTP handlers, `mu`, tight-scope `buf`, clear single-letter receivers.

```go
// WRONG
func join(re, str string) string { ... }

// CORRECT
func join(prefix, suffix string) string { ... }
```

**Artifacts:** Cursor — `.cursor/rules/naming-full-words.mdc`. Claude Code skill — `.claude/skills/naming-full-words.md` (use for refactors or name reviews).

---

## End-of-Phase Audit Rule

**Before closing any development phase, run `/audit` to verify quality, security, and correctness.**

Audit covers: build + tests pass, security checklist, error handling review, no TODOs left from previous phase, code coverage for new packages.

See the `audit` skill for the full checklist.

---

## Official Glossary (decided — do not mix)

| Term | Meaning in this project |
|------|------------------------|
| **tool** | Go function implementing `internal/tools.Tool` (read_file, bash, etc.) |
| **agent** | Profile with model + tool allowlist + permissionMode + system prompt |
| **skill** | Reusable Markdown prompt template (`.claude/skills/*.md`) |
| **command** | REPL **slash command** (`/help`, `/compact`, `/memory`, etc.); do not use this term for Cobra subcommands |
| **CLI command** / **subcommand** | Cobra entry (e.g. `goclaw`, `goclaw sessions list`); agent profile is selected via **`--profile`**, not a slash command |
| **hook** | Handler for an `EventType` — Go `On(...)`, or `external_hooks` / `.goclaw/hooks.json` (command / HTTP) |
| **session** | Conversation history; persisted to `~/.goclaw/sessions/<id>.jsonl` on exit; optional resume via `--session` |
| **memory** | Facts persisted across sessions (`~/.goclaw/memory/`) |

---

## Package Structure

```
goclaw/
├── cmd/goclaw/
│   ├── main.go                  ← slog + `cli.NewRootCmd` wiring `app.RunChat(..., fullscreenChat{})`
│   ├── tui.go                   ← Bubble Tea TUI (`FullscreenChatRunner`); keeps `internal/app` tests free of `ui/chat` import
│   └── version.go               ← `Version` (ldflags)
├── internal/
│   ├── cli/                     ← Cobra tree only (`root.go`: `NewRootCmd` with injected run funcs; tests avoid full UI link)
│   ├── app/
│   │   ├── run.go               ← `RunChat`, `RunListSessions`; delegates TUI to `FullscreenChatRunner`; readline REPL
│   │   ├── chat_wiring.go       ← `PrepareChatRuntime` (`ChatRuntime`): config, client, session, tools, MCP, hooks, orchestrator options
│   │   ├── repl_readline.go     ← readline REPL loop, tool approval prompt, `runOrchestratorTurn`
│   │   ├── terminal_sink.go     ← readline `StreamSink` implementation
│   │   ├── banner.go            ← startup banner (and related helpers)
│   │   └── mock.go              ← canned assistant stream for `--mock` / UI wiring tests
│   ├── slashcmd/                ← `/` slash handlers: `HandleSlash` (`slash.go`), `editor.go`, tests
│   ├── ui/chat/                 ← Bubble Tea fullscreen TUI (`--tui` / `GOCLAW_USE_TUI`): `chat.go`, `sink.go`, `theme.go`
│   ├── llm/                     ← Client interface + AnthropicClient + OllamaClient
│   │   ├── client.go            ← Client interface, Request, ToolSpec, Event types
│   │   ├── message.go           ← Message (text + ToolCalls / ToolResults)
│   │   ├── anthropic_wire.go    ← Maps messages to Anthropic content blocks
│   │   ├── ollama_wire.go       ← Expands tool turns for Ollama /api/chat
│   │   ├── anthropic.go         ← SSE streaming to /v1/messages
│   │   ├── ollama.go            ← NDJSON streaming to /api/chat
│   │   └── retry.go             ← HTTP retries / backoff (D22) for Anthropic and Ollama POSTs
│   ├── session/session.go       ← Session{ID, Messages[]}, Add / AddAssistant / AddToolResults
│   ├── orchestrator/            ← main loop: user → LLM → tools → repeat (32 iter / 64 tool calls)
│   │   ├── orchestrator.go      ← `Run` / `RunStreaming`, `Orchestrator`, options, session/profile helpers
│   │   ├── compaction.go        ← token estimate, `maybeCompact`, `ForceCompact`
│   │   ├── request.go           ← `buildRequest`, allowlist / ReadOnly tool filtering
│   │   └── tool_exec.go         ← `executeTool`, permissions + hooks + registry dispatch
│   ├── tools/
│   │   ├── registry.go          ← interface Tool, Registry{Get/Register/Specs}
│   │   ├── read_file.go, write_file.go, edit_file.go, glob.go, grep.go, bash.go, web_fetch.go, web_search.go, todo_write.go
│   │   └── limits.go, ssrf.go   ← shared caps / SSRF checks for web_fetch
│   ├── planfile/                ← workspace `.goclaw/plan.md` path, template, handoff message text
│   ├── todos/                   ← session task list store (todo_write)
│   ├── permissions/             ← Policy{Evaluate(toolName) Decision}
│   │   └── permissions.go       ← ModeAsk|ModeAllow|ModeDeny → DecisionAllow|DecisionDeny|DecisionAsk
│   ├── config/
│   │   ├── config.go            ← Config{…}, Default()
│   │   └── loader.go            ← Load: user/project settings.json + settings.local.json merge
│   ├── coordinator/             ← D16 hub-and-spoke coordinator: `spawn_agent` tool + `WorkerNotification`
│   ├── hooks/                   ← Registry + external command/HTTP + LoadHooksFile
│   ├── mcp/                     ← stdio JSON-RPC session, ToolAdapter → tools.Tool
│   ├── ide/                     ← optional localhost POST notifier (GOCLAW_IDE_NOTIFY_URL)
│   ├── agents/profile.go        ← Profile{Name, ModelOverride, ToolAllowlist, ReadOnly, SystemPrompt}
│   ├── memory/                  ← Filesystem store under ~/.goclaw/memory/, MEMORY.md index
│   └── ...
├── docs/                        ← D16 coordinator sketch and other design notes
└── testutil/mockserver/         ← HTTP mock for Anthropic /v1/messages (tests without API tokens)
```

**Rule**: each package has exactly one responsibility. Do not merge packages.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama server URL |
| `OLLAMA_MODEL` | `qwen2.5-coder:14b` | Local model name when `provider=ollama` |
| `ANTHROPIC_API_KEY` | — | Required when `provider=anthropic` |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Override for tests (mock server) |
| `GOCLAW_MODEL` | `claude-sonnet-4-6` | Anthropic model when `provider=anthropic` |
| `GOCLAW_DISABLE_TOOLS` | (empty) | Set to `1` to run without tools (same idea as `--no-tools`) |
| `GOCLAW_LOG` | `info` | `debug` / `warn` / `error` for slog level |
| `GOCLAW_USE_TUI` | (empty) | Set to `1` to use fullscreen Bubble Tea TUI (same as `--tui`; default is readline REPL on a TTY) |
| `GOCLAW_USE_READLINE` | (empty) | Set to `1` to force readline and disable TUI |
| `GOCLAW_IDE_NOTIFY_URL` | (empty) | Optional `http`/`https` URL with host `127.0.0.1`, `localhost`, or `::1` — best-effort POST after each tool ([`internal/ide`](internal/ide/notify.go)) |

**Config paths:**
- User: `~/.goclaw/settings.json` and `~/.goclaw/settings.local.json`
- Project: `.goclaw/settings.json` and `.goclaw/settings.local.json`
- Local files are machine-local; do not commit project `settings.local.json`.

**Merge order:** `config.Default()` (includes env vars) → user `settings.json` → project `settings.json` → user `settings.local.json` → project `settings.local.json` (each step overrides overlapping keys). Then CLI: **`goclaw --profile <name>`** overrides `agent_profile` only.

**CLI (session / tools / UI):**
- **`--session <id>`** — load history from `~/.goclaw/sessions/<id>.jsonl` (clear error if missing).
- **`--list-sessions`** — print saved session ids and exit (same as **`goclaw sessions list`**).
- **`--no-tools`** — do not register tools (chat-only; useful with models that hallucinate tool JSON).
- **`--tui`** — fullscreen Bubble Tea TUI; default interactive mode is **readline** with a `>` prompt (claw-style). Also **`GOCLAW_USE_TUI=1`**.
- **`--readline`** — force readline; disables TUI even if `GOCLAW_USE_TUI` is set.

**REPL slash commands** (do not go to the LLM): `/help` or `help` or `?`; `/session`; `/sessions` (list saved ids); `/quit` or `/exit` (save and exit); `/new` (save current JSONL, start empty session); `/save` (persist without exit); `/compact` (force compaction); `/profile <name>` (switch profile without restart); `/plan path|init|template`; `/apply-plan [path]` (load plan file, switch to `general-purpose`, run one orchestrator turn); `/memory list|add|delete`. Hooks `SessionStart` / `SessionEnd` fire when the REPL starts and exits.

**Plan → execute:** save a Markdown plan at `.goclaw/plan.md` (see [`internal/planfile/planfile.go`](internal/planfile/planfile.go)); use `/apply-plan` to hand off to full tools. D16 coordinator sketch: [`docs/D16_COORDINATOR_SKETCH.md`](docs/D16_COORDINATOR_SKETCH.md).

Example **`settings.json`:**

```json
{
  "provider": "ollama",
  "agent_profile": "explore",
  "ollama_model": "qwen2.5-coder:14b",
  "bash_timeout_sec": 120,
  "tool_permissions": {
    "read_file": "allow",
    "bash": "ask",
    "web_fetch": "ask"
  }
}
```

Optional key **`bash_timeout_sec`** (positive integer, max 3600): overrides the default bash tool timeout ([`internal/tools/limits.go`](internal/tools/limits.go) `BashTimeoutSec` = 30). Omitted or zero keeps the default.

Modes: `ask` (default; prompts on **stderr** — answer `y` / `yes`), `allow`, `deny`. Implementation: [`internal/config/loader.go`](internal/config/loader.go), [`internal/orchestrator/orchestrator.go`](internal/orchestrator/orchestrator.go) (`WithToolApprover`).

---

## Go Coding Conventions

### Required
- **Logging**: use `log/slog`, never `fmt.Println` for internal output
- **Errors**: always wrap with context: `fmt.Errorf("action: %w", err)`
- **Error strings**: lowercase, no punctuation at end (`"failed to open file"` not `"Failed to open file."`)
- **Error matching**: use `errors.Is` / `errors.As`, never `==` on error values
- **Interfaces first**: define the interface before the implementation
- **Context**: all IO receives `context.Context` as first parameter
- **Defer cleanup**: `defer resp.Body.Close()`, `defer cancel()` immediately after resource acquisition
- **Tool names exposed to the model**: `snake_case` (read_file, web_fetch)
- **Internal Go types**: `CamelCase` (ToolRegistry, AnthropicClient)
- **Interfaces**: base name or `-er` suffix (Client, Tool, Handler — not IClient, not ToolInterface)
- **Keep interfaces small**: 1–3 methods max; split if larger

### Forbidden
- `fmt.Println` for logging (use slog)
- `panic` except for unrecoverable programming errors (duplicate registration)
- `init()` functions
- Mutable global variables (use `sync.Once` for singletons)
- Ignoring errors with `_` in production code
- Returning concrete types when an interface is sufficient
- Capitalized error strings or error strings ending with punctuation

---

## Go Best Practices (curated from Uber Guide + Effective Go)

### Error handling
```go
// Wrap with context — always
return fmt.Errorf("orchestrator: stream: %w", err)

// Match errors structurally — never with ==
if errors.Is(err, ErrPermissionDenied) { ... }
if errors.As(err, &myErr) { ... }

// Sentinel errors at package level
var ErrPermissionDenied = errors.New("permission denied")
```

### Concurrency
```go
// Channel direction in signatures — always explicit
func produce(out chan<- Event)  { ... }
func consume(in <-chan Event)   { ... }

// Never leak goroutines — always provide an exit path
go func() {
    defer close(out)
    select {
    case out <- event:
    case <-ctx.Done():  // exit path
        return
    }
}()

// Protect shared state — sync.Mutex or dedicated goroutine
type Registry struct {
    mu    sync.Mutex
    tools map[string]Tool
}
```

### Testing
```go
// Table-driven tests — preferred pattern
func TestPermissionEvaluate(t *testing.T) {
    tests := []struct {
        name     string
        tool     string
        mode     Mode
        want     Decision
    }{
        {"allow mode", "bash", ModeAllow, DecisionAllow},
        {"deny mode",  "bash", ModeDeny,  DecisionDeny},
        {"ask mode",   "bash", ModeAsk,   DecisionAsk},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := NewPolicy()
            p.Set(tt.tool, tt.mode)
            if got := p.Evaluate(tt.tool); got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}

// Use t.TempDir() — auto-cleaned after test
func TestReadFile(t *testing.T) {
    dir := t.TempDir()
    // ...
}

// Use t.Helper() in test helpers
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil { t.Fatalf("unexpected error: %v", err) }
}

// Race detector — run CI with -race flag
// go test -race ./...
```

### Dependency injection over globals
```go
// CORRECT — inject dependencies
func New(cfg Config, client llm.Client, sess *session.Session) *Orchestrator

// WRONG — hidden global dependency
var globalClient llm.Client
func Run() { globalClient.Stream(...) }
```

### Interface satisfaction check (compile-time)
```go
// Verify at compile time that the type implements the interface
var _ Tool = (*ReadFileTool)(nil)
var _ llm.Client = (*AnthropicClient)(nil)
```

---

## Tool Contract MVP

### Nine built-in tools (MVP)

| Tool | Risk | Input | Output cap | Notes |
|------|------|-------|------------|-------|
| `read_file` | read_only | path, offset?, limit? | 512 KiB or 200 lines | Rejects symlinks outside workspace |
| `glob` | read_only | pattern | 500 paths | Workspace-scoped; no `..` |
| `grep` | read_only | pattern, path? | 200 matches, 512 KiB/file | RE2 regex; skips binaries |
| `bash` | shell | command, cwd? | 256 KiB truncated | Allowlist (D4); single simple command — `rejectShellMetacharacters` blocks pipes, `;`, `&&`, redirects, `$(...)`, unquoted `&`; timeout from `bash_timeout_sec` or default 30s |
| `write_file` | write | path, content | 1 MiB content | Atomic (temp+rename); parent dir must exist; stripped from ReadOnly |
| `edit_file` | write | path, old_string, new_string, replace_all? | 1 MiB result | str_replace; exact match unless replace_all:true; preserves file mode; stripped from ReadOnly |
| `web_search` | network | query | Top 8 hits, 2 KiB/snippet | DuckDuckGo JSON |
| `web_fetch` | network | url, max_bytes? | 1 MiB text/HTML | SSRF guards required |
| `todo_write` | session_meta | merge, todos[] | 50 items; 500 runes/content | Session task list; snapshot in system prompt via `orchestrator.WithTodoStore`; cleared on `/new` |

### Message / tool protocol
- Session messages use `llm.Message`: plain `role` + `content`, or assistant turns with `tool_calls`, or user turns with `tool_results` (Anthropic-style). The orchestrator records **all** tool calls from one assistant stream, then one user message with **all** matching results.
- **Anthropic:** history maps to `tool_use` / `tool_result` content blocks (see `anthropic_wire.go`).
- **Ollama:** assistant turns use `tool_calls` with `type: "function"`; tool outputs must use the **`tool_name`** field on `role: "tool"` messages (not `name`). See [Ollama tool calling](https://docs.ollama.com/capabilities/tool-calling) — wrong field breaks the round-trip (model may print fake JSON instead of seeing results).
- **Recoverable failures** (permission deny, user decline, unknown tool, hook block, tool error): surfaced as `tool_result` with `is_error`, and the LLM loop continues.
- **Fatal** errors: LLM stream failure, iteration limit, tool call budget, or missing approver when the policy requires Ask.

### Agent loop budgets (do not change without consensus)
- **Max LLM iterations per user message**: 32
- **Max tool calls per user message**: 64
- **bash timeout**: 30s default, configurable
- **web_fetch timeout**: 30s

### SSRF — blocked targets in `web_fetch`
- `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` (RFC1918)
- `127.0.0.0/8` (loopback)
- `169.254.169.254` (AWS/GCP metadata)
- `::1` and IPv6 link-local
- Re-validate after each redirect (max 5 hops)

---

## Architecture Decisions (D1–D22 condensed)

| Decision | Actionable rule |
|----------|----------------|
| D1: Providers | Ollama = default. Anthropic = opt-in with API key. Do not couple provider logic. |
| D4: Bash security | Allowlist (expanded binaries + git subcommands), not denylist. Single simple command: `rejectShellMetacharacters` blocks shell chaining that would bypass the allowlist. User confirmation always required in Ask mode. |
| D5: Permissions | Default = ModeAsk (fail-closed). `bypassPermissions` NOT implemented in MVP. |
| D6: MCP | **stdio + Streamable HTTP** (`internal/mcp`, `http.go`): subprocess **or** `mcp_servers[].url` (optional `headers`); tools as `mcp__server__tool`. HTTP URLs must be **loopback** unless `mcp_allow_remote_urls` is set in settings. **Not done:** OAuth, WebSocket, legacy HTTP+SSE-only servers. |
| D7: Config paths | `~/.goclaw/` (user), `.goclaw/` (project). Decided, do not change. |
| D12: Dedicated tools | Prefer `read_file`/`glob`/`grep` over bash equivalents. Bash = last resort. |
| D13: Memory | Filesystem at `~/.goclaw/memory/`. 4 types: user/feedback/project/reference. |
| D15: Compaction | Threshold as configurable fraction (default 0.85). Session size for compaction uses a **token estimate** (chars/4 heuristic, capped) mapped against a budget derived from `defaultContextBudgetChars`; no live provider token APIs in MVP. |
| D16: Multi-agent | **Done** — `internal/coordinator`: `spawn_agent` tool, `Coordinator` profile (allowlist: spawn_agent + todo_write), isolated worker sessions via `session.New()`, `WorkerNotification` JSON result, nesting prevention. Team/Swarm remains Phase 3+. |
| D17: YOLO Classifier | **Implemented** in `internal/permissions/risk.go` — rule-based risk scorer (0–100); `yolo_threshold: -1` default (off); auto-approves reads at threshold 0. |
| D18: Hooks | PreToolUse can block. PostToolUse is best-effort (non-fatal). |
| D19: Custom agents | **Implemented** — Markdown + YAML frontmatter in `~/.goclaw/agents/*.md` and `.goclaw/agents/*.md`; fields: `name`, `model`, `tool_allowlist`, `read_only`, `system_prompt`; body appended to system prompt; hot-reload on `/profile`; project overrides user overrides built-in. See [`internal/agents/profile.go`](internal/agents/profile.go). |
| D22: Retry | **Implemented** in [`internal/llm/retry.go`](internal/llm/retry.go): `doHTTPWithRetry` wraps Anthropic and Ollama POSTs — retries on **429**, **503**, **504** with `Retry-After` when present, else exponential backoff (base 500ms, ceiling 5min), up to **10** attempts; transient **network errors** retry with the same backoff. |

---

## What NOT to do

- **Do not implement `bypassPermissions`** — dangerous without YOLO Classifier (v2+)
- **Do not use denylist for bash** — use a narrow allowlist (ls, cat, go, git basics)
- **Do not merge packages** — each `internal/X` has a single responsibility
- **Do not hardcode context sizes** — use a configurable fraction
- **Do not use `fmt.Println`** for logging
- **Do not commit `.goclaw/settings.local.json`** — it is machine-local
- **Do not assume Ollama is running** — handle connection refused with a clear error
- **Do not point MCP HTTP at non-loopback URLs** unless the user explicitly sets `mcp_allow_remote_urls` (SSRF posture); **do not add OAuth** for MCP without a dedicated security pass
- **Do not mix session (RAM) with memory (disk)** — they are separate systems
- **Do not write Spanish** in code, comments, or commit messages — English only

---

## How to test

```bash
# Full build (must be clean)
go build ./...

# Unit tests
go test ./...

# Race detector (requires CGO; on Windows use WSL/Linux CI or install a C toolchain)
# go test -race ./...

# Tests with mock server (no API tokens)
ANTHROPIC_BASE_URL=http://localhost:PORT go test ./...

# Manual run against local Ollama
OLLAMA_HOST=http://localhost:11434 OLLAMA_MODEL=qwen2.5-coder:14b go run ./cmd/goclaw
```

### Non-interactive smoke

No TTY required — use before a release or when CI cannot drive the full REPL:

1. **Binary and session store:** from the module root, `go run ./cmd/goclaw --list-sessions` or `go run ./cmd/goclaw sessions list` must exit 0 (prints ids or `(no saved sessions)`). Same for `go build -o goclaw ./cmd/goclaw && ./goclaw sessions list`.
2. **Chat-only path:** `GOCLAW_DISABLE_TOOLS=1 go run ./cmd/goclaw --no-tools` starts the REPL; with Ollama down you should still see a clear connection error after sending one line, not a silent hang on startup.
3. **Full REPL (manual, TTY):** run without `--no-tools`, press ↑ for history, trigger a tool in Ask mode and confirm the `Allow execution?` prompt uses readline editing.

**Mock server** in `testutil/mockserver/`:
- Start with `mockserver.New(scenarios)` → returns `*Server` with `.URL`
- Use `ANTHROPIC_BASE_URL=server.URL` to point the client at the mock
- Scenarios match on the last message fingerprint (plain text or `tool_result` bodies); use `Tool` for one tool or `Tools` for multi-tool streams. See `internal/orchestrator/*_test.go` for examples.

---

## Roadmap — current status

```
[DONE] Phase 0: go.mod, package layout, mock server, OllamaClient, AnthropicClient
[DONE] Phase 1: Core loop — session UUID, JSONL store + rotation, REPL + slog, mock tests
[DONE] Phase 2: Tools MVP — read_file (workspace), bash (allowlist), web_fetch (SSRF), web_search (DDG)
[DONE] Phase 3: Ask prompt (stderr), `-profile`, `settings.json` loader, `tool_permissions` map
[DONE] Phase 4: Memory filesystem + index (MEMORY.md), compaction + memory snippet in system prompt (MVP)
[DONE] Refinement: structured tool messages (multi-tool, `is_error` continuation), `settings.local.json`,
       `-session` / `-list-sessions`, `-no-tools` / `GOCLAW_DISABLE_TOOLS`, Ollama `tool_name` wire fix
[DONE] write_file + edit_file: workspace-scoped atomic writes, str_replace edit, ReadOnly profile stripping
[DONE] Post-MVP slice: MCP stdio client + multi-server config, MCP tools on Registry, external hooks + workspace trust, IDE localhost notifier (`GOCLAW_IDE_NOTIFY_URL`)
[DONE] v2: YOLO Classifier (`internal/permissions/risk.go`), multi-agent coordinator (`internal/coordinator`), custom agents (`internal/agents/profile.go`), parallel tool execution, LLM-driven compaction, script tool
[POST-MVP] v3+: Plugins, deeper IDE/editor parity (extension-side), Team/Swarm — see [IDE_BRIDGE.md](../IDE_BRIDGE.md)
```

### To continue (polish / post-MVP):
1. ~~`go test -race` in CI~~ — [`.github/workflows/goclaw-ci.yml`](../.github/workflows/goclaw-ci.yml) at monorepo root (`defaults.run.working-directory: goclaw`). Subfolder workflows under `goclaw/.github/` are not executed by GitHub Actions.
2. ~~`web_search` MVP fallback~~ — DDG JSON fields `Answer`, `Definition`, `Results` (HTML stripped), empty-query message with `duckduckgo.com` link; optional later: non-DDG backend.
3. ~~Compaction + tests~~ — summary lists removed/tail counts; [`internal/orchestrator/estimate_test.go`](internal/orchestrator/estimate_test.go) and extra edge tests.
4. ~~REPL slash UX~~ — `/help`, `/session`, `/sessions`, `/quit`/`/exit`, `/new`, `/save`, `/memory`, `/compact`; short id prompt.
5. ~~Compaction threshold tokens~~ — `sessionTokenEstimate` + `contextBudgetTokens` (provider-tuned char÷N heuristic, not live tokenizer APIs).
6. ~~D22 HTTP retries~~ — [`internal/llm/retry.go`](internal/llm/retry.go) + tests.
7. ~~README + hooks logging~~ — [`README.md`](README.md); post-tool hook handler errors logged with `slog.WarnContext` in [`internal/hooks/hooks.go`](internal/hooks/hooks.go).
8. ~~`glob` / `grep` tools~~ — workspace-scoped ([`internal/tools/glob.go`](internal/tools/glob.go), [`grep.go`](internal/tools/grep.go)); explore/plan allowlists updated.
9. ~~`write_file` / `edit_file` tools~~ — atomic writes, str_replace, ReadOnly stripping; [`internal/tools/write_file.go`](internal/tools/write_file.go), [`edit_file.go`](internal/tools/edit_file.go).
10. ~~REPL readline + expanded bash allowlist~~ — [`github.com/chzyer/readline`](https://github.com/chzyer/readline) in [`internal/app/repl_readline.go`](internal/app/repl_readline.go); allowlist in [`internal/tools/bash.go`](internal/tools/bash.go).
11. ~~Bash single-command shell policy~~ — [`rejectShellMetacharacters`](internal/tools/bash.go) blocks pipes, `;`, `&&`, redirects, subshells, `$(...)`, and unquoted `&` (URLs with query strings must be quoted).
12. ~~`bash_timeout_sec` in settings~~ — [`internal/config/loader.go`](internal/config/loader.go); [`NewBashWithTimeout`](internal/tools/bash.go).
13. ~~Clearer Ollama dial errors~~ — [`wrapOllamaDialErr`](internal/llm/ollama.go) on connection refused.
14. ~~D12 in base system prompt~~ — dedicated tools before bash; [`internal/orchestrator/request.go`](internal/orchestrator/request.go) `baseSystemPrompt`.
15. Further optional: LLM-written compaction text; provider token APIs; stdin smoke test in CI.

When adding sections to this file, keep them in English (Language Rule — STRICT). Cursor rules (`.cursor/rules/*.mdc`) and agent skills (`.claude/skills/*.md`) are also maintained in English.

---

## Post-MVP phased delivery (MCP, IDE, hooks)

**Status (2026-04):** MCP-1–3 plus **Streamable HTTP** (`internal/mcp/http.go`), external hooks + workspace trust, IDE **lockfile discovery** (`ide_bridge_mcp`, `internal/ide/discovery.go`), and **minimal** IDE notify (`GOCLAW_IDE_NOTIFY_URL`). **Not done:** MCP OAuth, WebSocket transport, async hook transports.

| Phase | Scope | Status |
|-------|--------|--------|
| **MCP-1** | stdio client, process lifecycle, JSON-RPC framing | **Done** — `internal/mcp`, tests with piped mock |
| **MCP-1b** | Streamable HTTP client (POST JSON + optional SSE) | **Done** — `internal/mcp/http.go`; loopback default, `mcp_allow_remote_urls` opt-in |
| **MCP-2** | Tool discovery → `mcp__server__tool` on `Registry` | **Done** — `mcp.ToolAdapter`, [`internal/app/chat_wiring.go`](internal/app/chat_wiring.go) (`RegisterSessionTools`), permissions |
| **MCP-3** | Multiple servers, config merge by `id` | **Done** — `mcp_servers` in loader; failed server isolated |
| **IDE** | Localhost MCP toward editor | **Partial** — lockfile → `mcp_servers` when `ide_bridge_mcp`; `GOCLAW_IDE_NOTIFY_URL`; see [IDE_BRIDGE.md](../IDE_BRIDGE.md) (**D21**) |
| **Hooks** | Subprocess / HTTP + project file | **Done** — `external_hooks`, `.goclaw/hooks.json` + `trusted_workspace` |

**Future epics (no code until scoped):** plugins (**PLUGINS.md**) — track as a **separate planning pass**. D16 coordinator (**done** — `internal/coordinator`), D17 YOLO classifier (**done** — `internal/permissions/risk.go`), D19 custom agents (**done** — `internal/agents/profile.go`) are all implemented.

Further IDE work: extension publishers adopt `~/.goclaw/ide/*.json` (or document alternate paths) and exercise the same MCP HTTP path as other remote servers.
