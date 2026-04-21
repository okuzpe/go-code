# agentdemo (smoke / UI sandbox)

This binary is a **minimal** Bubble Tea + Ollama streaming sandbox. It does **not** run the goclaw tool registry, orchestrator, or permissions stack.

For a local-first coding agent with tools, sessions, and the production TUI, use the main CLI:

```bash
goclaw
```

from a project directory (see `docs/goclaw/usage.md`). Do not extend agentdemo with new agent features; keep changes limited to smoke tests or TUI experiments intended to be ported into `internal/ui/chat`.
