# docs/archive

Archived and **path-sensitive** documentation. From this folder, **`../`** reaches `docs/` siblings; **`../../goclaw/`** reaches the Go module at the repo root (not `docs/goclaw/`).

**GoClaw (shipped behavior):** [goclaw/CLAUDE.md](../../goclaw/CLAUDE.md)  
**Project README:** [goclaw/README.md](../../goclaw/README.md)  
**English navigation hub:** [architecture.md](../architecture.md)  
**Full doc index:** [docs-map.md](../docs-map.md)

## In this folder

| File | Role |
|------|------|
| [architecture-legacy-es.md](./architecture-legacy-es.md) | Long-form **Spanish** architecture draft (**§1–§8**). In-repo links target `../reference/`, `../goclaw/`, etc. |

## Monorepo reference specs (from here: use `../reference/`)

| Topic | File |
|------|------|
| Agent profiles | [agent-profiles.md](../reference/agent-profiles.md) |
| Bash / shell security | [bash-security.md](../reference/bash-security.md) |
| Context / compaction | [context-compaction.md](../reference/context-compaction.md) |
| Coordinator vs swarm | [coordinator-mode.md](../reference/coordinator-mode.md) |
| API costs | [costs.md](../reference/costs.md) |
| Custom Markdown agents | [custom-agents.md](../reference/custom-agents.md) |
| Documentation map | [docs-map.md](../docs-map.md) |
| Go vs Rust | [go-vs-rust-assistant.md](../reference/go-vs-rust-assistant.md) |
| Hooks | [hooks.md](../reference/hooks.md) |
| IDE / bridge | [ide-bridge.md](../reference/ide-bridge.md) |
| Local models | [local-models.md](../reference/local-models.md) |
| MCP | [mcp.md](../reference/mcp.md) |
| Memory | [memory-system.md](../reference/memory-system.md) |
| OpenClaw — agents/tools | [openclaw-agents-tools.md](../openclaw/openclaw-agents-tools.md) |
| OpenClaw — gateway | [openclaw-gateway-channels.md](../openclaw/openclaw-gateway-channels.md) |
| OpenClaw — repo map | [openclaw-reference.md](../openclaw/openclaw-reference.md) |
| Plugins | [plugins.md](../reference/plugins.md) |
| Practical tips | [practical-tips.md](../reference/practical-tips.md) |
| External link list | [references.md](../reference/references.md) |
| LLM retries | [retry-logic.md](../reference/retry-logic.md) |
| Skills | [skills.md](../reference/skills.md) |
| Tool contract | [tool-contract.md](../reference/tool-contract.md) |
| YOLO / auto-mode classifier | [yolo-classifier.md](../reference/yolo-classifier.md) |

## Changelog

| Date | Change |
|------|--------|
| 2026-04-10 | Added README; fixed relative links for browsing from `docs/archive/`. |
| 2026-04-10 | Index table points at `docs/reference/` kebab-case files. |
| 2026-04-10 | Fixed module links: use `../../goclaw/` for `CLAUDE.md` / `README.md` (module root), not `../goclaw/` (that is `docs/goclaw/`). |
