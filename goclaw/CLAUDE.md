# GoClaw — Project Rules for AI Agents

## Identity
- **Project**: `goclaw` — Go CLI agent, equivalent to Claude Code, focused on local models via Ollama
- **Go module**: `github.com/okuzpe/goclaw`
- **Go version**: 1.26
- **Default provider**: local Ollama (`qwen2.5-coder:14b`)
- **Alternative providers**: Anthropic API (`ANTHROPIC_API_KEY`); **OpenAI-compatible** HTTP APIs (`provider: "openai_compatible"`, e.g. OpenRouter, Groq, LM Studio local server — `OPENAI_BASE_URL`, `OPENAI_API_KEY`, `OPENAI_MODEL`)
- **Repo root**: clone path + `/goclaw` (module root for `go build` / `go test`)

**Workspace note:** If the parent folder also contains `claw-code/`, treat it as **reference material only**. It is not part of this module, not covered by this roadmap, and must not be modified when implementing goclaw. All product code, tests, and phase work live under `goclaw/`.

**Documentation map:** [documentation.md](../docs/goclaw/documentation.md) (what belongs in `goclaw/` vs monorepo `docs/`). **Canonical index:** [README.md](README.md); master file list: [docs-map.md](../docs/docs-map.md). **Docs ↔ code layers:** [code-adjustment-map.md](../docs/reference/code-adjustment-map.md).

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

## Multi-agent model (coordinator vs swarm vs external)

| Layer | What goclaw does |
|-------|------------------|
| **Hub-and-spoke coordinator** | Implemented: default `agent_profile` is `coordinator` (hub); tools `spawn_agent`, `stop_task`, `todo_write` only on the parent; workers use `general-purpose`, `explore`, `plan`, or `verification` with isolated `session.Session`. Optional `spawn_agent` **`interactive: true`** plus REPL **`/focus`** / **`/detach`**. Worker runs stream to the parent UI via [`ContextWithStreamSink`](internal/orchestrator/sink_context.go). If multiple tools are auto-approved in one message, **`spawn_agent` is not parallelized** with other tools ([`pendingToolsIncludeSpawnAgent`](internal/orchestrator/orchestrator.go)) to avoid duplicate workers and GPU contention. Prefer **`general-purpose`** for direct repo edits without delegation. End-user notes: [usage.md — Agent profiles](../docs/goclaw/usage.md) (`timeout_sec`, coordinator vs `general-purpose`). Code: [`internal/coordinator`](internal/coordinator/), wiring in [`internal/app/chat_wiring.go`](internal/app/chat_wiring.go). Design notes: [`coordinator.md`](../docs/goclaw/coordinator.md), product comparison [`coordinator-mode.md`](../docs/reference/coordinator-mode.md). |
| **Team/Swarm (peer agents)** | **Minimal disk hub** — [`internal/swarm`](internal/swarm/): mailboxes under a user-chosen directory (tests + future tools). Not the same as `spawn_agent`; see [`swarm.md`](../docs/goclaw/swarm.md). |
| **External orchestration** | Optional: wrap `goclaw` with your own scheduler/event bus (analogous in spirit to claw-code + clawhip + Discord). Not a goclaw dependency. |

**Deferred until there is a concrete consumer:** structured worker lifecycle events (for external routers) and OAuth / `login` flows for Anthropic. Neither is required for the Ollama-first workflow; add them when an integration needs a stable event schema or token storage.

---

## Package Structure

```
goclaw/
├── cmd/goclaw/
│   ├── main.go                  ← slog + `cli.NewRootCmd` wiring `app.RunChat` + `app.RunPrompt` + `fullscreenChat{}`
│   ├── tui.go                   ← Bubble Tea TUI (`FullscreenChatRunner`); keeps `internal/app` tests free of `ui/chat` import
│   └── version.go               ← `Version` (ldflags)
├── internal/
│   ├── cli/                     ← Cobra tree only (`root.go`: `NewRootCmd` with injected run funcs; tests avoid full UI link)
│   ├── app/
│   │   ├── run.go               ← `RunChat`, `RunListSessions`; default on TTY = Bubble Tea TUI via `FullscreenChatRunner`; readline when opted out (`--readline`, `GOCLAW_USE_TUI=0`, …); `printStartupBanner` only when not using fullscreen TUI
│   │   ├── chat_wiring.go       ← `PrepareChatRuntime` (`ChatRuntime`): config, client, session, tools, MCP, plugins, skills, hooks, orchestrator options
│   │   ├── repl_readline.go     ← readline REPL loop, tool approval prompt, `runOrchestratorTurn`
│   │   ├── terminal_sink.go     ← readline `StreamSink` implementation
│   │   ├── banner.go            ← readline / non-TTY startup banner (`printStartupBanner`); not used for default TUI
│   │   └── mock.go              ← canned assistant stream for `--mock` / UI wiring tests
│   ├── slashcmd/                ← `/` slash handlers: `HandleSlash` (`slash.go`), `editor.go`, tests
│   ├── ui/chat/                 ← Bubble Tea fullscreen TUI (`--tui` / `GOCLAW_USE_TUI`): `chat.go`, `sink.go`, `theme.go`
│   ├── llm/                     ← Client interface + AnthropicClient + OllamaClient + OpenAICompatClient
│   │   ├── client.go            ← Client interface, Request, ToolSpec, Event types
│   │   ├── message.go           ← Message (text + ToolCalls / ToolResults)
│   │   ├── anthropic_wire.go    ← Maps messages to Anthropic content blocks
│   │   ├── ollama_wire.go       ← Expands tool turns for Ollama /api/chat
│   │   ├── anthropic.go         ← SSE streaming to /v1/messages
│   │   ├── ollama.go            ← NDJSON streaming to /api/chat
│   │   ├── openai_compat.go     ← SSE streaming to /v1/chat/completions (OpenAI-compatible)
│   │   └── retry.go             ← HTTP retries / backoff (D22) for LLM POSTs
│   ├── session/session.go       ← Session{ID, Messages[]}, Add / AddAssistant / AddToolResults
│   ├── orchestrator/            ← main loop: user → LLM → tools → repeat (32 iter / 64 tool calls)
│   │   ├── orchestrator.go      ← `Run` / `RunStreaming`, `Orchestrator`, options, session/profile helpers
│   │   ├── compaction.go        ← token estimate, `maybeCompact`, `ForceCompact`
│   │   ├── request.go           ← `buildRequest`, allowlist / ReadOnly tool filtering
│   │   ├── user_language_hint.go ← merge heuristic + whatlanggo + preferred_response_language / from_os
│   │   ├── user_language_locale.go ← LANG / LC_* primary tag fallback
│   │   ├── user_language_whatlang.go ← whatlanggo reliable detection → es/en/fr/de/pt
│   │   └── tool_exec.go         ← `executeTool`, permissions + hooks + registry dispatch
│   ├── tools/
│   │   ├── registry.go          ← interface Tool, Registry{Get/Register/Specs}
│   │   ├── read_file.go, write_file.go, edit_file.go, patch.go, glob.go, grep.go, bash.go, web_fetch.go, web_search.go, todo_write.go
│   │   └── limits.go, ssrf.go   ← shared caps / SSRF checks for web_fetch
│   ├── planfile/                ← workspace `.goclaw/plan.md` path, template, handoff message text
│   ├── todos/                   ← session task list store (todo_write)
│   ├── permissions/             ← Policy{Evaluate(toolName) Decision}
│   │   └── permissions.go       ← ModeAsk|ModeAllow|ModeDeny → DecisionAllow|DecisionDeny|DecisionAsk
│   ├── config/
│   │   ├── config.go            ← Config{…}, Default()
│   │   └── loader.go            ← Load: user/project settings.json + settings.local.json merge
│   ├── coordinator/             ← D16 hub-and-spoke coordinator: `spawn_agent` tool + `WorkerNotification`
│   ├── swarm/                   ← V3+ disk mailboxes (peer messaging), separate from coordinator
│   ├── plugin/                  ← V3 local plugins: `goclaw-plugin.json`, optional hooks file, allow/deny
│   ├── skills/                  ← V3 SKILL.md discovery for system prompt injection
│   ├── hooks/                   ← Registry + external command/HTTP + LoadHooksFile
│   ├── mcp/                     ← stdio JSON-RPC session, ToolAdapter → tools.Tool
│   ├── ide/                     ← optional localhost POST notifier (GOCLAW_IDE_NOTIFY_URL)
│   ├── agents/profile.go        ← Profile{Name, ModelOverride, ToolAllowlist, ReadOnly, SystemPrompt}
│   ├── memory/                  ← Filesystem store under ~/.goclaw/memory/, MEMORY.md index
│   └── ...
└── testutil/mockserver/         ← HTTP mock for Anthropic /v1/messages (tests without API tokens)
```

**Topic docs (monorepo):** [`docs/goclaw/`](../docs/goclaw/) — coordinator wire format, MCP remote notes, swarm, QA checklists (listed in [README.md](README.md)).

**Rule**: each package has exactly one responsibility. Do not merge packages.

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama server URL |
| `OLLAMA_MODEL` | `qwen2.5-coder:14b` | Local model name when `provider=ollama` |
| `GOCLAW_COMPACTION_MODEL` | — | Optional model id for LLM-driven compaction only; applied in `config.Default()` before settings merge; a `compaction_model` key in merged `settings.json` overrides; empty keeps main model for compaction |
| `GOCLAW_TASK_MODEL_ROUTER` | — | Per-turn model routing: `off`, `rules` (heuristics), or `llm` (classifier call); needs non-empty **`task_models`** in merged settings; see [`docs/goclaw/model-routing.md`](../docs/goclaw/model-routing.md) |
| `GOCLAW_TASK_MODEL_ROUTER_MODEL` | — | Model id for the `llm` router’s short JSON reply only; merged **`task_model_router_model`** overrides; empty uses **`ModelForCompaction()`** |
| `GOCLAW_PREFERRED_RESPONSE_LANGUAGE` | — | Optional UI reply bias for runtime language hints: `auto`, `from_os`, or `es` / `en` / `fr` / `de` / `pt`; merged `preferred_response_language` in settings overrides |
| `ANTHROPIC_API_KEY` | — | Required when `provider=anthropic` |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Override for tests (mock server) |
| `GOCLAW_MODEL` | `claude-sonnet-4-6` | Anthropic model when `provider=anthropic`; short aliases `opus`, `sonnet`, `haiku` resolve to full ids (see [`internal/config/config.go`](internal/config/config.go)) |
| `OPENAI_BASE_URL` | — | Base URL including `/v1` when `provider=openai_compatible` (e.g. `https://openrouter.ai/api/v1`); or `openai_base_url` in settings |
| `OPENAI_API_KEY` | — | API key for the OpenAI-compatible endpoint; or `openai_api_key` in settings |
| `OPENAI_MODEL` | — | Model id for `provider=openai_compatible` (e.g. `openrouter/free`); or `openai_model` in settings |
| `GOCLAW_DISABLE_TOOLS` | (empty) | Set to `1` to run without tools (same idea as `--no-tools`) |
| `GOCLAW_MOCK_FAST` | (empty) | Set to `1` to remove pacing delays from `--mock` (CI / scripts) |
| `BRAVE_SEARCH_API_KEY` | — | Brave Search API token when `web_search_backend` is `brave` (optional; can use `brave_search_api_key` in settings) |
| `SERPAPI_API_KEY` | — | SerpAPI key when `web_search_backend` is `serpapi` (optional; can use `serpapi_api_key` in settings) |
| `GOCLAW_LOG` | `info` | `debug` / `warn` / `error` for slog level |
| `GOCLAW_USE_TUI` | (empty) | `1` = force TUI; **`0` = opt out of default TUI** and use readline on a TTY |
| `GOCLAW_USE_READLINE` | (empty) | Set to `1` to force readline REPL (disables default TUI) |
| `GOCLAW_AGENT_PROFILE` | (empty) | When set, overrides `agent_profile` from settings (e.g. `general-purpose`); **`--profile` still wins** |
| `GOCLAW_IDE_NOTIFY_URL` | (empty) | Optional `http`/`https` URL with host `127.0.0.1`, `localhost`, or `::1` — best-effort POST after each tool ([`internal/ide`](internal/ide/notify.go)) |

**Config paths:**
- User: `~/.goclaw/settings.json` and `~/.goclaw/settings.local.json`
- Project: `.goclaw/settings.json` and `.goclaw/settings.local.json`
- Local files are machine-local; do not commit project `settings.local.json`.

**Merge order:** `config.Default()` (includes env vars) → user `settings.json` → project `settings.json` → user `settings.local.json` → project `settings.local.json` (each step overrides overlapping keys). Then **`GOCLAW_AGENT_PROFILE`** if set. Then CLI: **`goclaw --profile <name>`** overrides `agent_profile` last; non-empty **`--task-model-router`** overrides merged **`task_model_router`**.

**Provider and model (summary):**

| `provider` | Model source | Custom agents (`model` in YAML frontmatter) |
|------------|--------------|-----------------------------------------------|
| `ollama` (default) | `OLLAMA_MODEL` / `ollama_model` | Overrides global model when non-empty |
| `anthropic` | `GOCLAW_MODEL` / aliases | Overrides global model when non-empty |
| `openai_compatible` | `OPENAI_MODEL` / `openai_model` | Overrides global model when non-empty |

**CLI (session / tools / UI):**
- **`--session <id>`** — load history from `~/.goclaw/sessions/<id>.jsonl` (clear error if missing).
- **`--list-sessions`** — print saved session ids and exit (same as **`goclaw sessions list`**).
- **`--no-tools`** — do not register tools (chat-only; useful with models that hallucinate tool JSON).
- **`--tui`** — fullscreen Bubble Tea TUI (**default on a TTY**). Also **`GOCLAW_USE_TUI=1`** to force; **`GOCLAW_USE_TUI=0`** opts out to readline.
- **`--readline`** — force line-at-a-time readline REPL (disables default TUI).
- **`--mock`** — stream a canned assistant reply without calling the model (UI check; use with `GOCLAW_MOCK_FAST=1` in automation).
- **`--task-model-router off|rules|llm`** — override per-turn **`task_models`** routing mode for this process (requires a configured **`task_models`** map when not `off`); see [`model-routing.md`](../docs/goclaw/model-routing.md).
- **`--output-format text|json`** — for one-shot stdout: `text` prints the final assistant message; `json` prints `{"response","toolCalls"}` (same shape as `--json-output`).
- **`--json-output`** — shorthand for `--output-format json` when piping one line on stdin (no REPL; incompatible with explicit **`--tui`** / **`GOCLAW_USE_TUI=1`**; set `tool_permissions` to `allow` for tools you need without prompts).
- **`goclaw prompt "…"`** — same one-turn loop using argv instead of stdin; respects `--output-format` / `--json-output`.

**REPL slash commands** (do not go to the LLM): `/help` or `help` or `?`; `/session`; `/sessions` (list saved ids); `/quit` or `/exit` (save and exit); `/new` (save current JSONL, start empty session); `/save` (persist without exit); `/compact` (force compaction); `/profile <name>` (switch profile without restart); `/workers` (interactive `spawn_agent` workers); `/focus <task_id_prefix>` or `/focus parent`; `/detach` (back to coordinator); `/plan path|init|save|template`; `/apply-plan [path]` (load plan file, switch to `general-purpose`, stream one execution turn via modelSubmit); `/memory list|add|delete`. **Sends to the LLM via modelSubmit:** `/btw <text>` (side question — rewrites one user message with a brief-aside preamble). Hooks `SessionStart` / `SessionEnd` fire when the REPL starts and exits.

**Default `agent_profile`:** `coordinator` (hub). Use `agent_profile`, `GOCLAW_AGENT_PROFILE`, or `--profile general-purpose` for direct coding with file tools on the main session.

**Plan → execute:** `/profile plan` → ask for a plan → `/plan save` (saves last assistant message to `.goclaw/plan.md`) → `/apply-plan` (streams execution via the normal REPL turn; switches to `general-purpose`). See [`internal/planfile/planfile.go`](internal/planfile/planfile.go). D16 coordinator sketch: [`coordinator.md`](../docs/goclaw/coordinator.md).

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
  },
  "web_search_backend": "brave",
  "brave_search_api_key": "your-token",
  "web_search_fallback_ddg": true,
  "token_count_mode": "auto"
}
```

Optional keys: **`preferred_response_language`** — `auto` (default), `from_os`, or `es` / `en` / `fr` / `de` / `pt` (steers runtime user-language hint; see [`docs/goclaw/i18n.md`](../docs/goclaw/i18n.md)); **`compaction_model`** — model id for LLM summarization when **`llm_compaction`** is true (smaller/faster model than the main turn); **`task_model_router`** / **`task_models`** / **`task_model_router_model`** — per-turn model selection (`off` \| `rules` \| `llm`); see [`model-routing.md`](../docs/goclaw/model-routing.md); **`web_search_backend`** (`ddg` \| `brave` \| `serpapi`), **`brave_search_api_key`**, **`serpapi_api_key`**, **`web_search_fallback_ddg`** (default true when using a non-DDG backend), **`token_count_mode`** (`auto` \| `heuristic`) for Anthropic compaction (`auto` uses the [count_tokens API](https://docs.anthropic.com/en/api/messages-count-tokens) once the heuristic estimate crosses 70% of the compaction threshold).

**Open-weight stack (Ollama):** see [`docs/goclaw/ollama-stack.md`](../docs/goclaw/ollama-stack.md) for project template agents under `goclaw/.goclaw/agents/` and `OLLAMA_MAX_LOADED_MODELS` notes.

**OpenAI-compatible example** (e.g. [OpenRouter](https://openrouter.ai); base URL must include `/v1`):

```json
{
  "provider": "openai_compatible",
  "openai_base_url": "https://openrouter.ai/api/v1",
  "openai_api_key": "sk-or-v1-…",
  "openai_model": "openrouter/free",
  "model_context_tokens": 128000
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

### Built-in tools (MVP)

| Tool | Risk | Input | Output cap | Notes |
|------|------|-------|------------|-------|
| `read_file` | read_only | path, offset?, limit? | 512 KiB or 200 lines | Rejects symlinks outside workspace |
| `glob` | read_only | pattern | 500 paths | Workspace-scoped; no `..` |
| `grep` | read_only | pattern, path? | 200 matches, 512 KiB/file | RE2 regex; skips binaries |
| `bash` | shell | command, cwd? | 256 KiB truncated | Allowlist (D4); single simple command — `rejectShellMetacharacters` blocks pipes, `;`, `&&`, redirects, `$(...)`, unquoted `&`; timeout from `bash_timeout_sec` or default 30s |
| `write_file` | write | path, content | 1 MiB content | Atomic (temp+rename); parent dir must exist; stripped from ReadOnly |
| `edit_file` | write | path, old_string, new_string, replace_all? | 1 MiB result | str_replace; exact match unless replace_all:true; preserves file mode; stripped from ReadOnly |
| `patch` | write | path, diff | 1 MiB diff; 1 MiB result | Git/unified diff, exactly one file; `path` must match `a/` / `b/` headers; binary rejected; stripped from ReadOnly |
| `web_search` | network | query | Top 8 hits, 2 KiB/snippet | Backend from `web_search_backend` (DDG / Brave / SerpAPI; optional DDG fallback). DDG uses instant JSON first, then HTML SERP when JSON has no hits |
| `web_fetch` | network | url, max_bytes? | 1 MiB text/HTML | SSRF guards; HTML responses reduced to plain text when extraction yields enough words |
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
| D1: Providers | Ollama = default. Anthropic and `openai_compatible` (OpenAI Chat Completions–compatible) = opt-in with credentials. Keep provider-specific code behind `llm.Client`. |
| D4: Bash security | Allowlist (expanded binaries + git subcommands), not denylist. Single simple command: `rejectShellMetacharacters` blocks shell chaining that would bypass the allowlist. User confirmation always required in Ask mode. |
| D5: Permissions | Default = ModeAsk (fail-closed). `bypassPermissions` NOT implemented in MVP. |
| D6: MCP | **stdio + Streamable HTTP** (`internal/mcp`, `http.go`): subprocess **or** `mcp_servers[].url` (optional `headers`, optional **`bearer_token_file`** → `Authorization: Bearer` if no header set); tools as `mcp__server__tool`. HTTP URLs must be **loopback** unless `mcp_allow_remote_urls` is set in settings. **Not done:** OAuth, WebSocket, legacy HTTP+SSE-only servers. See [`mcp-remote.md`](../docs/goclaw/mcp-remote.md). |
| D7: Config paths | `~/.goclaw/` (user), `.goclaw/` (project). Decided, do not change. |
| D12: Dedicated tools | Prefer `read_file`/`glob`/`grep` over bash equivalents. Bash = last resort. |
| D13: Memory | Filesystem at `~/.goclaw/memory/`. 4 types: user/feedback/project/reference. Opt-in **auto-capture** (`memory_auto_extract: true`): after successful `write_file` / `edit_file`, one-line project entry (path only), capped per session — [`internal/memory/autocapture.go`](internal/memory/autocapture.go). |
| D15: Compaction | Threshold as configurable fraction (default 0.85). Session size uses a **heuristic token estimate** (chars÷N by provider) against `model_context_tokens` / provider defaults. When `provider=anthropic` and `token_count_mode` is `auto` (default), goclaw calls Anthropic **`POST /v1/messages/count_tokens`** once the heuristic crosses 70% of the compaction threshold; on failure it falls back to the heuristic. |
| D16: Multi-agent | **Done** — `internal/coordinator`: `spawn_agent` and `stop_task` tools, `Coordinator` profile (allowlist: spawn_agent, stop_task, todo_write), isolated worker sessions via `session.New()`, `WorkerNotification` JSON result, nesting prevention. **Swarm** (separate): `internal/swarm` disk hub — [`swarm.md`](../docs/goclaw/swarm.md). |
| D17: YOLO Classifier | **Implemented** in `internal/permissions/risk.go` — rule-based risk scorer (0–100); `yolo_threshold: -1` default (off); auto-approves reads at threshold 0. |
| D18: Hooks | PreToolUse can block. PostToolUse is best-effort (non-fatal). |
| D19: Custom agents | **Implemented** — Markdown + YAML frontmatter in `~/.goclaw/agents/*.md` and `.goclaw/agents/*.md`; fields: `name`, `model`, `tool_allowlist`, `read_only`, `system_prompt`; body appended to system prompt; hot-reload on `/profile`; project overrides user overrides built-in. See [`internal/agents/profile.go`](internal/agents/profile.go). |
| D20: Plugins | **MVP** — `internal/plugin`: each root contains `goclaw-plugin.json` (`name`, optional `hooks_file`); `plugin_dirs` in settings + `--plugin-dir` (repeatable); `plugin_allow` / `plugin_deny` (deny wins). Hooks load via same mechanism as `.goclaw/hooks.json`. Treat plugins as **executable config** (supply chain), like `trusted_workspace`. |
| Skills (runtime) | **`internal/skills`** discovers `SKILL.md` under `.goclaw/skills/`, `.claude/skills/`, `~/.goclaw/skills/`, `~/.goclaw/.claude/skills/` (recursive under each root); content is injected under `## Loaded skills (SKILL.md)` in the system prompt (bounded size). |
| D22: Retry | **Implemented** in [`internal/llm/retry.go`](internal/llm/retry.go): `doHTTPWithRetry` wraps Anthropic, Ollama, and OpenAI-compat POSTs — retries on **429**, **503**, **504** with `Retry-After` when present, else exponential backoff (base 500ms, ceiling 5min), up to **10** attempts; transient **network errors** retry with the same backoff. |

---

## What NOT to do

- **Do not implement `bypassPermissions`** — dangerous without YOLO Classifier (v2+)
- **Do not use denylist for bash** — use a narrow allowlist (ls, cat, go, git basics)
- **Do not merge packages** — each `internal/X` has a single responsibility
- **Do not hardcode context sizes** — use a configurable fraction
- **Do not use `fmt.Println`** for logging
- **Do not commit `.goclaw/settings.local.json`** — it is machine-local
- **Do not assume Ollama is running** — handle connection refused with a clear error
- **Do not point MCP HTTP at non-loopback URLs** unless the user explicitly sets `mcp_allow_remote_urls` (SSRF posture); **do not add OAuth** for MCP without a dedicated security pass (see [`mcp-remote.md`](../docs/goclaw/mcp-remote.md))
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
3. **Stdin + mock (CI):** `GOCLAW_MOCK_FAST=1 printf 'ping\n' | go run ./cmd/goclaw --no-tools --mock --readline` exits 0 without a live LLM (Linux CI; readline + pipe can be flaky on some Windows shells).
4. **JSON one-shot:** `printf 'hello\n' | go run ./cmd/goclaw --output-format json --no-tools` prints one JSON object on stdout (`--json-output` is equivalent; use `--mock` for a canned response without the provider). **Text one-shot:** `go run ./cmd/goclaw prompt "hello" --no-tools`.
5. **Full REPL (manual, TTY):** run without `--no-tools`, press ↑ for history, trigger a tool in Ask mode; in **readline** mode confirm the `Allow execution?` prompt uses readline editing; in **TUI** confirm the approval modal above the input.

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
[DONE] v3 slice: local plugins (`internal/plugin`), SKILL.md prompt injection (`internal/skills`), MCP `bearer_token_file`, opt-in memory auto-capture, minimal swarm hub (`internal/swarm`), IDE extension contract §7 in [ide-bridge.md](../docs/reference/ide-bridge.md). **Still open:** MCP OAuth/WS, remote plugin marketplace, full editor MCP UX.
```

### To continue (polish / post-MVP):
1. ~~`go test -race` in CI~~ — [`.github/workflows/goclaw-ci.yml`](../.github/workflows/goclaw-ci.yml) at monorepo root (`defaults.run.working-directory: goclaw`). Subfolder workflows under `goclaw/.github/` are not executed by GitHub Actions.
2. ~~`web_search` backends~~ — `web_search_backend` (`ddg` \| `brave` \| `serpapi`), optional DDG fallback; keys in settings / env (`BRAVE_SEARCH_API_KEY`, `SERPAPI_API_KEY`).
3. ~~Compaction + tests~~ — summary lists removed/tail counts; [`internal/orchestrator/estimate_test.go`](internal/orchestrator/estimate_test.go) and extra edge tests.
4. ~~REPL slash UX~~ — `/help`, `/session`, `/sessions`, `/quit`/`/exit`, `/new`, `/save`, `/memory`, `/compact`; short id prompt.
5. ~~Compaction threshold tokens~~ — heuristic + Anthropic `count_tokens` when `token_count_mode` is `auto` (see D15).
6. ~~D22 HTTP retries~~ — [`internal/llm/retry.go`](internal/llm/retry.go) + tests.
7. ~~README + hooks logging~~ — [`README.md`](README.md); post-tool hook handler errors logged with `slog.WarnContext` in [`internal/hooks/hooks.go`](internal/hooks/hooks.go).
8. ~~`glob` / `grep` tools~~ — workspace-scoped ([`internal/tools/glob.go`](internal/tools/glob.go), [`grep.go`](internal/tools/grep.go)); explore/plan allowlists updated.
9. ~~`write_file` / `edit_file` tools~~ — atomic writes, str_replace, ReadOnly stripping; [`internal/tools/write_file.go`](internal/tools/write_file.go), [`edit_file.go`](internal/tools/edit_file.go).
10. ~~REPL readline + expanded bash allowlist~~ — [`github.com/chzyer/readline`](https://github.com/chzyer/readline) in [`internal/app/repl_readline.go`](internal/app/repl_readline.go); allowlist in [`internal/tools/bash.go`](internal/tools/bash.go).
11. ~~Bash single-command shell policy~~ — [`rejectShellMetacharacters`](internal/tools/bash.go) blocks pipes, `;`, `&&`, redirects, subshells, `$(...)`, and unquoted `&` (URLs with query strings must be quoted).
12. ~~`bash_timeout_sec` in settings~~ — [`internal/config/loader.go`](internal/config/loader.go); [`NewBashWithTimeout`](internal/tools/bash.go).
13. ~~Clearer Ollama dial errors~~ — [`wrapOllamaDialErr`](internal/llm/ollama.go) on connection refused.
14. ~~D12 in base system prompt~~ — dedicated tools before bash; [`internal/orchestrator/request.go`](internal/orchestrator/request.go) `baseSystemPrompt`.
15. ~~stdin smoke test in CI~~ — `.github/workflows/goclaw-ci.yml` runs `printf 'ping\n' \| go run ... --no-tools --mock --readline` with `GOCLAW_MOCK_FAST=1` on Ubuntu.

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
| **IDE** | Localhost MCP toward editor | **Partial** — lockfile → `mcp_servers` when `ide_bridge_mcp`; `GOCLAW_IDE_NOTIFY_URL`; see [ide-bridge.md](../docs/reference/ide-bridge.md) (**D21**) |
| **Hooks** | Subprocess / HTTP + project file | **Done** — `external_hooks`, `.goclaw/hooks.json` + `trusted_workspace` |

**Plugins (D20):** MVP implemented — see D20 row; full marketplace / remote install remains out of scope. D16 coordinator (**done** — `internal/coordinator`), D17 YOLO classifier (**done** — `internal/permissions/risk.go`), D19 custom agents (**done** — `internal/agents/profile.go`).

Further IDE work: extension publishers adopt `~/.goclaw/ide/*.json` (or document alternate paths) and exercise the same MCP HTTP path as other remote servers.
