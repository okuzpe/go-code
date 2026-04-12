# Plugins (modular packages) — reference and Go mapping

**Status in goclaw:** **D20** local plugins **shipped** in [`goclaw/internal/plugin`](../../goclaw/internal/plugin); remote marketplace **not implemented**. See [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). Below mixes **reference-product** breadth with what goclaw actually loads today.

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (D20 plugins). Reference (third-party, Claude Code analysis): [Plugins — claude-code-explain](https://claude-code-explain.helmcode.com/plugins).

A **plugin** is a **package** with a mandatory manifest (in the reference `.claude-plugin/plugin.json`) that can contribute, in combination: **commands** (/slash), **skills**, **agent** `.md` files, **output styles**, **hooks**, **MCP**, **LSP**, **channels** (advanced assistant modes), **settings**, and **`userConfig`** (options on enable). The rest is **auto-detected** by folder convention.

---

## 1. Fit in our architecture

| Existing layer | How it relates |
|----------------|----------------|
| [custom-agents.md](./custom-agents.md) | Plugin `agents/` folder → same `.md` + frontmatter; **plugin** priority in the cascade; security restrictions on plugin agents (see §7 in that doc). |
| [hooks.md](./hooks.md) | `hooks/hooks.json` or inline in the manifest; top-priority source in the hooks §3 table. |
| §2.9 **Skills** | `skills/*/SKILL.md` inside the plugin. |
| **MCP** (D6) | `.mcp.json` or `mcpServers` key; prefixes like `plugin:name:server`; deduplication: **manual config wins** in reference. Client/naming/scopes detail: [mcp.md](./mcp.md). |
| [philosophy.md](../goclaw/philosophy.md#lessons-from-wider-agent-stacks) | Products with npm hubs like **ClawHub** (e.g. OpenClaw) share the **supply chain** risk of a marketplace; goclaw prioritizes MCP + local plugins. |
| **Permissions / enterprise** | `allowedPlugins`, `deniedPlugins` (deny wins); `strictPluginOnlyCustomization` blocks "manual" MCP if plugin-only — **D20** policy. |

**Value of implementing:** a single packaging format for teams that want to distribute a **complete flow** (skill + hook + agent + MCP) without asking the user to copy five paths. **Cost:** third-party downloads, cross-plugin dependency resolution, and attack surface (**supply chain**).

---

## 2. Directory structure (reference)

```
plugin-root/
├── .claude-plugin/
│   ├── plugin.json          # REQUIRED
│   └── marketplace.json    # publishers
├── commands/*.md
├── agents/*.md
├── skills/*/SKILL.md
├── output-styles/*.md
├── hooks/hooks.json
└── .mcp.json
```

**Capabilities (~9 types):** commands, agents, skills, outputStyles, hooks, mcpServers, lspServers, channels, settings (`userConfig` separate).

---

## 3. Sources and storage (reference)

| Origin | Example |
|--------|---------|
| Marketplace | `plugin@marketplace` |
| Session / dev | `--plugin-dir` (path) |
| Built-in | `plugin@builtin` (in the analyzed source the built-in registry may be empty) |

Cache: `~/.claude/plugins/cache/...`; enabled plugins and `pluginConfigs` in `settings.json`.

**Go mapping:** `--plugin-dir` flags first; cache under `~/.goclaw/plugins/`; `enabledPlugins` list in config (**D7**/**D20**).

---

## 4. Marketplace (reference, summary)

Source formats: relative path, npm, pip, git URL, GitHub, git subdirectory. **Anti-impersonation:** known marketplaces; cross-marketplace dependencies behind an explicit flag. `extraKnownMarketplaces` in settings.

**Go mapping:** remote marketplace is **not in the binary**; only local load / `--plugin-dir` and allow/deny policies.

---

## 5. Load flow (conceptual)

1. Resolve plugins (parallel): marketplace + session + built-in.  
2. **Merge** by name (session can win per ref.); **policy** overrides everything.  
3. Check dependencies; disable if missing.  
4. Per plugin: read manifest, auto-detect dirs, load hooks/MCP.  
5. **Register** capabilities in the global registry (MCP-derived tools, commands, agent profiles).

**Go mapping:** `internal/plugin` with `LoadAll(ctx, PluginLoadRequest) (*PluginBundle, error)` that returns **merged** contributions toward `tools`, `mcp`, `agentprofile`, `hooks`, `skills` without each package importing the orchestrator circularly — **main** or `config` assembles.

---

## 6. userConfig and variables

Options declared in the manifest → prompt on enable → values in `pluginConfigs` (non-sensitive) or secrets store (sensitive). Substitution `${user_config.KEY}` and env `CLAUDE_PLUGIN_OPTION_*` for hooks/commands.

**Go mapping:** `plugin.ConfigSchema` + validation; aligned with secrets handling in [CLAUDE.md](../../goclaw/CLAUDE.md) and [security.md](../goclaw/security.md).

---

## 7. `/plugin` command (reference)

Install, manage, validate manifest, marketplaces. **Go mapping:** `goclaw plugin …` subcommands when the feature exists; for now: only `--plugin-dir` for development.

---

## 8. Current state vs possible extensions

| Scope | Plugins |
|-------|---------|
| **goclaw today** | No remote marketplace; **`--plugin-dir`**, `plugin_dirs` / allow and deny lists, `goclaw-plugin.json` manifest, merge of skills / hooks / agents per **D20** |
| **Not implemented** | npm/remote marketplace, automatic updates, cross-plugin dependencies as in the reference product |
| **Future (if prioritized)** | More auto-detect + manifest parity with §2; see [roadmap.md](../goclaw/roadmap.md) optional waves |

---

## 9. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: 9 capabilities, layout, marketplace, policy, Go mapping, [plugins (helmcode)](https://claude-code-explain.helmcode.com/plugins) link |
| 2026-04-12 | Translated from Spanish to English |
