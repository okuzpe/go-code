# References (assistant / agentic CLI patterns)

**Índice de cobertura ↔ [Claude Code Internals](https://claude-code-explain.helmcode.com/):** [DOCS_MAP.md](DOCS_MAP.md) — tabla por tema, alcance MVP, orden de lectura para implementar. **Auditoría de todos los `.md` del repo (por carpetas y política):** [ARCHITECTURE.md §8.0](ARCHITECTURE.md).

**Decisiones y arquitectura (trabajo en curso):** [ARCHITECTURE.md](ARCHITECTURE.md) — mapa conceptual §3, auditoría del corpus `.md` §8, modelos locales §2 ter.

- [SKILLS.md](SKILLS.md) — formato `SKILL.md`, hooks en sesión, roadmap v3.
- [BASH_SECURITY.md](BASH_SECURITY.md) — capas shell/sandbox (referencia vs MVP), **D4**.
- [COSTS.md](COSTS.md) — pricing cloud y modo “fast”; **D1**; N/A Ollama.
- [LOCAL_MODELS.md](LOCAL_MODELS.md) — Ollama, modelos 7B/14B, límites RTX 4050 / 32 GB, imagen/vídeo como herramientas opcionales.
- [TOOL_CONTRACT.md](TOOL_CONTRACT.md) — nombres/límites/riesgo de herramientas MVP, política red, presupuesto bucle (**antes de codificar** `internal/tools`).
- [GO_VS_RUST_ASSISTANT.md](GO_VS_RUST_ASSISTANT.md) — Go vs Rust para asistente/agente CLI (resumen + enlaces).
- [AGENT_PROFILES.md](AGENT_PROFILES.md) — perfiles de agente (modelo + tools + permisos + contexto), eco Go.
- [MEMORY_SYSTEM.md](MEMORY_SYSTEM.md) — memoria entre sesiones (tipos, MEMORY.md, límites, extractor), eco Go.
- [CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md) — ventana de contexto, micro-compact, auto-compact, presupuestos de salida, eco Go (**D15**).
- [COORDINATOR_MODE.md](COORDINATOR_MODE.md) — Coordinator hub-and-spoke vs Team/Swarm, mailboxes, invariante prompts autocontenidos (**D16**).
- [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) — monitor de auto-modo: consulta lateral LLM, XML dos etapas, fast paths, fail-closed (**D17**).
- [HOOKS.md](HOOKS.md) — eventos (~27), tipos command/prompt/agent/http, permisos, workspace trust (**D18**).
- [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) — agentes `*.md` + YAML (tools, MCP, hooks, memoria por agente), prioridad vs built-ins (**D19**).
- [PLUGINS.md](PLUGINS.md) — manifiesto, nueve tipos de capacidad, marketplace, merge al arranque, políticas (**D20**).
- [IDE_BRIDGE.md](IDE_BRIDGE.md) — integración IDE local (MCP localhost, lockfiles) vs Bridge remoto; prioridad editor; **D21**.
- [MCP.md](MCP.md) — cliente MCP hacia servidores externos (stdio/SSE/HTTP/WS), naming `mcp__*`, scopes, auth, roadmap v2/v3; **D6**.
- [PRACTICAL_TIPS.md](PRACTICAL_TIPS.md) — diez decisiones de producto visibles (memoria, compact, permisos, perfiles, costes `/fast`); eco Go.
- [RETRY_LOGIC.md](RETRY_LOGIC.md) — backoff exponencial, 429/529/5xx, reintentos por invocación, modo unattended (referencia); **D22**.

**OpenClaw** (producto upstream **Node/TS**; en este workspace **solo** estos resúmenes — el código está en [github.com/openclaw/openclaw](https://github.com/openclaw/openclaw)):

- [OPENCLAW_REFERENCE.md](OPENCLAW_REFERENCE.md) — mapa del monorepo y tabla `src/` → eco Go
- [OPENCLAW_GATEWAY_CHANNELS.md](OPENCLAW_GATEWAY_CHANNELS.md) — gateway, daemon, canales
- [OPENCLAW_AGENTS_AND_TOOLS.md](OPENCLAW_AGENTS_AND_TOOLS.md) — agentes, web, tubería

**Otro código en local:** [claw-code/](claw-code/) (parity / Rust / TUI — ver [ARCHITECTURE.md §8.0](ARCHITECTURE.md) Tier 2).

- [Anthropic API documentation](https://docs.anthropic.com/en/api/getting-started)
- [How Claude Code works](https://code.claude.com/docs/en/how-claude-code-works) (official product docs)
- [Model Context Protocol specification](https://modelcontextprotocol.io/specification/2025-11-25)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [Ollama](https://ollama.com/) (ejecución local de modelos, API HTTP)
- [Open-source image generation models (2026 overview)](https://www.bentoml.com/blog/a-guide-to-open-source-image-generation-models) — FLUX, SD, Z-Image, Qwen-Image, ComfyUI vs A1111; profundidad en [LOCAL_MODELS.md §3](LOCAL_MODELS.md)
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
