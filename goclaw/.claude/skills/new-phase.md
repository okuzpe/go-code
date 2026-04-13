---
name: new-phase
description: Use when starting a new development phase or asking what comes next in goclaw.
---

## Starting a new phase in goclaw

### Pre-flight checklist (before any phase)

```bash
go build ./...
# go test ./...   # only when the user asks — CI otherwise
grep -rE "TODO (Phase|Fase)" ./internal/  # should be empty (legacy Spanish tag + English)
```

Fix failures before continuing.

---

### Phase 1 — Core loop
**Goal**: REPL that talks to Ollama without tools.

Touch:
- `internal/session/session.go` — UUID + JSONL persistence
- `cmd/goclaw/main.go` — `log/slog`, signal handling (Ctrl+C)
- `testutil/mockopenai/` — OpenAI-style `/v1/chat/completions` mock for tests

Deliverable: `go run ./cmd/goclaw` returns text via local Ollama.  
Tests: mock scenarios (text-only, streaming, HTTP 500).

---

### Phase 2 — Core tools
**Goal**: Four tools with basic safety.

Add:
- `internal/tools/read_file.go`, `glob.go`, `grep.go`, `bash.go`, `web_fetch.go`, `web_search.go`

Register in `cmd/goclaw/main.go`.

Deliverable: agent can read files, run allowlisted shell, fetch URLs.  
Tests: read roundtrip, bash stdout, SSRF blocked, etc.

**Security before marking done**: symlink escape (`security-review`), bash allowlist + timeout, web_fetch SSRF.

---

### Phase 3 — Permissions + agents + config
**Goal**: Ask mode, profiles, JSON settings.

Touch:
- `internal/permissions/permissions.go`
- `internal/agents/profile.go`
- `internal/config/config.go`, `internal/config/loader.go` — defaults → user + project `settings.json` / `settings.local.json` → CLI flags

Deliverable: Ask prompts on stderr; read-only profiles block bash.

---

### Phase 4 — Memory + hooks + compaction
**Goal**: Durable facts, hook API, context compaction.

Implement:
- `internal/memory/memory.go` — Save / Load / List / Delete
- `internal/memory/index.go` — regenerate `MEMORY.md`
- Session rotation lives in `internal/session/store.go` (not a separate `jsonl.go`)
- Orchestrator compaction using `cfg.AutoCompactThreshold`; optional `WithMemoryStore` for system-prompt snippet

Deliverable: data under `~/.goclaw/memory/`; long sessions compact; hooks fire around tool use.

---

### v2/v3 extensions (all shipped)

| Track | Ships as | Status |
|-------|----------|--------|
| v2: MCP | `internal/mcp` stdio + HTTP | **Done** |
| v2: YOLO Classifier | `internal/permissions/risk.go` | **Done** |
| v2: Multi-agent | `internal/coordinator` (`spawn_agent`, `stop_task`) | **Done** |
| v3: Plugins | `internal/plugin` (local only; no remote marketplace) | **Done** |
| v3: IDE Bridge | lockfile MCP + `GOCLAW_IDE_NOTIFY_URL` | **Partial** (full editor UX extension-dependent) |
