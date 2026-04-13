# References (assistant / agentic CLI patterns)

**Coverage index ↔ [Claude Code Internals](https://claude-code-explain.helmcode.com/):** [docs-map.md](../docs-map.md) — table by topic, what is **implemented** in goclaw vs reference, reading order.

**Decisions and architecture:** [architecture.md](../architecture.md) — short hub (navigation). GoClaw behavior: [goclaw/CLAUDE.md](../../goclaw/CLAUDE.md). **Docs ↔ code (maintenance):** [code-adjustment-map.md](./code-adjustment-map.md) — what to read and which `internal/*` packages to touch per layer.

- [skills.md](./skills.md) — `SKILL.md` format, session hooks, roadmap v3.
- [bash-security.md](./bash-security.md) — shell/sandbox layers (reference vs current implementation), **D4**.
- [costs.md](./costs.md) — cloud pricing and "fast" mode; **D1**; N/A Ollama.
- [local-models.md](./local-models.md) — Ollama, 7B/14B models, RTX 4050 / 32 GB limits, image/video as optional tools.
- [tool-contract.md](./tool-contract.md) — tool names/limits/risk (contract **implemented** in `internal/tools`), network policy, loop budget.
- [agent-profiles.md](./agent-profiles.md) — agent profiles (model + tools + permissions + context), Go mapping.
- [memory-system.md](./memory-system.md) — cross-session memory (types, MEMORY.md, limits, extractor), Go mapping.
- [context-compaction.md](./context-compaction.md) — context window, micro-compact, auto-compact, output budgets, Go mapping (**D15**).
- [coordinator-mode.md](./coordinator-mode.md) — Coordinator hub-and-spoke vs Team/Swarm, mailboxes, self-contained prompt invariant (**D16**).
- [yolo-classifier.md](./yolo-classifier.md) — auto-mode monitor: lateral LLM call, two-stage XML, fast paths, fail-closed (**D17**).
- [hooks.md](./hooks.md) — events (~27), types command/prompt/agent/http, permissions, workspace trust (**D18**).
- [custom-agents.md](./custom-agents.md) — `*.md` + YAML agents (tools, MCP, hooks, per-agent memory), priority vs built-ins (**D19**).
- [plugins.md](./plugins.md) — manifest, nine capability types, marketplace, merge on startup, policies (**D20**).
- [ide-bridge.md](./ide-bridge.md) — local IDE integration (MCP localhost, lockfiles) vs remote Bridge; editor priority; **D21**.
- [mcp.md](./mcp.md) — MCP client toward external servers (stdio/SSE/HTTP/WS), `mcp__*` naming, scopes, auth, roadmap v2/v3; **D6**.
- [practical-tips.md](./practical-tips.md) — ten visible product decisions (memory, compact, permissions, profiles, `/fast` costs); Go mapping.
- [retry-logic.md](./retry-logic.md) — exponential backoff, 429/529/5xx, retries per invocation, unattended mode (reference); **D22**.

**OpenClaw** (upstream **Node/TS** multi-channel agent product — useful for comparison only): [github.com/openclaw/openclaw](https://github.com/openclaw/openclaw), [docs.openclaw.ai](https://docs.openclaw.ai). Design lessons absorbed into goclaw docs: [philosophy.md — Lessons from wider agent stacks](../goclaw/philosophy.md#lessons-from-wider-agent-stacks), [roadmap.md — Future transport and scale](../goclaw/roadmap.md#future-transport-and-scale).

**Other local code:** [claw-code/](../../claw-code/) (parity / Rust / TUI — see [roadmap.md — Future transport and scale](../goclaw/roadmap.md#future-transport-and-scale) and [CLAUDE.md](../../goclaw/CLAUDE.md)).

- [Ollama REST API](https://github.com/ollama/ollama/blob/main/docs/api.md) (local `/api/chat` used by the CLI)
- [How Claude Code works](https://code.claude.com/docs/en/how-claude-code-works) (official product docs)
- [Model Context Protocol specification](https://modelcontextprotocol.io/specification/2025-11-25)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [Ollama](https://ollama.com/) (local model execution, HTTP API)
- [Open-source image generation models (2026 overview)](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models) — FLUX, SD, Z-Image, Qwen-Image, ComfyUI vs A1111; depth in [local-models.md §3](./local-models.md)
- [Claude Code architecture deep dive (third-party)](https://wavespeed.ai/blog/posts/claude-code-architecture-leaked-source-deep-dive/)
- [Claude Code internals (third-party explainer)](https://claude-code-explain.helmcode.com/)
- [Claude Code internals — System Prompt (third-party)](https://claude-code-explain.helmcode.com/system-prompt)
- [Claude Code internals — Tools (third-party)](https://claude-code-explain.helmcode.com/tools)
- [Claude Code internals — Agents (third-party)](https://claude-code-explain.helmcode.com/agents)
- [Claude Code internals — Memory (third-party)](https://claude-code-explain.helmcode.com/memory)
- [Claude Code internals — Permissions (third-party)](https://claude-code-explain.helmcode.com/permissions)
- [Claude Code internals — Context & Compaction (third-party)](https://claude-code-explain.helmcode.com/context-compaction)
- [Claude Code internals — Coordinator Mode (third-party)](https://claude-code-explain.helmcode.com/coordinator-mode)
- [Claude Code internals — YOLO Classifier (third-party)](https://claude-code-explain.helmcode.com/yolo-classifier)
- [Claude Code internals — Hooks (third-party)](https://claude-code-explain.helmcode.com/hooks)
- [Claude Code internals — Custom Agents (third-party)](https://claude-code-explain.helmcode.com/custom-agents)
- [Claude Code internals — Plugins (third-party)](https://claude-code-explain.helmcode.com/plugins)
- [Claude Code internals — Bridge & IDE (third-party)](https://claude-code-explain.helmcode.com/bridge-ide)
- [Claude Code internals — MCP (third-party)](https://claude-code-explain.helmcode.com/mcp)
- [Claude Code internals — Practical Tips (third-party)](https://claude-code-explain.helmcode.com/tips)
- [Claude Code internals — Costs (third-party)](https://claude-code-explain.helmcode.com/costs)
- [Claude Code internals — Retry Logic (third-party)](https://claude-code-explain.helmcode.com/retry-logic)
- [Claude Code internals — Proactive Mode / KAIROS (third-party)](https://claude-code-explain.helmcode.com/proactive-mode)
- [Claude Code internals — Hidden Features (third-party)](https://claude-code-explain.helmcode.com/hidden-features)
- [Claude Code internals — Skills (third-party)](https://claude-code-explain.helmcode.com/skills)
- [Claude Code internals — Slash Commands (third-party)](https://claude-code-explain.helmcode.com/slash-commands)
- [Claude Code internals — Bash Security (third-party)](https://claude-code-explain.helmcode.com/bash-security)
