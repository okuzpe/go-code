# Plugins (paquetes modulares) — referencia y eco Go

**Status in goclaw:** **D20** local plugins **shipped** in [`goclaw/internal/plugin`](../../goclaw/internal/plugin); remote marketplace **not implemented**. See [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). Below mixes **reference-product** breadth with what goclaw actually loads today.

Profundidad ligada a [CLAUDE.md](../../goclaw/CLAUDE.md) (D20 plugins). Referencia (terceros, análisis de Claude Code): [Plugins — claude-code-explain](https://claude-code-explain.helmcode.com/plugins).

Un **plugin** es un **paquete** con manifiesto obligatorio (en referencia `.claude-plugin/plugin.json`) que puede aportar, de forma combinada: **comandos** (/slash), **skills**, **agentes** `.md`, **output styles**, **hooks**, **MCP**, **LSP**, **canales** (modos asistente avanzados), **ajustes** y **`userConfig`** (opciones al habilitar). El resto se **auto-detecta** por convención de carpetas.

---

## 1. Cabida en nuestra arquitectura

| Capa existente | Cómo se relaciona |
|----------------|-------------------|
| [custom-agents.md](./custom-agents.md) | Carpeta `agents/` del plugin → mismos `.md` + frontmatter; prioridad **plugin** en la cascada; restricciones de seguridad en agentes de plugin (ver §7 en ese doc). |
| [hooks.md](./hooks.md) | `hooks/hooks.json` o inline en manifiesto; fuente prioritaria en la tabla de §3 de hooks. |
| §2.9 **Skills** | `skills/*/SKILL.md` dentro del plugin. |
| **MCP** (D6) | `.mcp.json` o clave `mcpServers`; prefijos tipo `plugin:nombre:servidor`; deduplicación: **config manual gana** en referencia. Detalle de cliente/naming/scopes: [mcp.md](./mcp.md). |
| [philosophy.md](../goclaw/philosophy.md#lessons-from-wider-agent-stacks) | Productos con hubs npm tipo **ClawHub** (p. ej. OpenClaw) comparten el riesgo de **supply chain** de un marketplace; goclaw prioriza MCP + plugins locales. |
| **Permisos / empresa** | `allowedPlugins`, `deniedPlugins` (deny gana); `strictPluginOnlyCustomization` bloquea MCP “manual” si solo plugins — política **D20**. |

**Valor para implementar:** un solo formato de empaquetado para equipos que quieren distribuir **flujo completo** (skill + hook + agente + MCP) sin pedir al usuario que copie cinco rutas. **Coste:** descarga de terceros, resolución de dependencias entre plugins, y superficie de ataque (**supply chain**).

---

## 2. Estructura de directorio (referencia)

```
plugin-root/
├── .claude-plugin/
│   ├── plugin.json          # REQUERIDO
│   └── marketplace.json    # publicadores
├── commands/*.md
├── agents/*.md
├── skills/*/SKILL.md
├── output-styles/*.md
├── hooks/hooks.json
└── .mcp.json
```

**Capabilities (~9 tipos):** commands, agents, skills, outputStyles, hooks, mcpServers, lspServers, channels, settings (`userConfig` aparte).

---

## 3. Fuentes y almacenamiento (referencia)

| Origen | Ejemplo |
|--------|---------|
| Marketplace | `plugin@marketplace` |
| Sesión / dev | `--plugin-dir` (path) |
| Built-in | `plugin@builtin` (en fuente analizada el registro built-in puede estar vacío) |

Cache: `~/.claude/plugins/cache/...`; habilitados y `pluginConfigs` en `settings.json`.

**Eco Go:** flags `--plugin-dir` primero; cache bajo `~/.config/assistant/plugins/`; lista `enabledPlugins` en config (**D7**/**D20**).

---

## 4. Marketplace (referencia, resumen)

Formatos de fuente: path relativo, npm, pip, git URL, GitHub, subdirectorio git. **Anti-suplantación:** marketplaces conocidos; dependencias cross-marketplace tras flag explícito. `extraKnownMarketplaces` en settings.

**Eco Go:** marketplace remoto **no está en el binario**; solo carga local / `--plugin-dir` y políticas allow/deny.

---

## 5. Flujo de carga (conceptual)

1. Resolver plugins (paralelo): marketplace + sesión + built-in.  
2. **Merge** por nombre (sesión puede ganar según ref.); **política** gana a todo.  
3. Comprobar dependencias; deshabilitar si faltan.  
4. Por plugin: leer manifiesto, auto-detectar dirs, cargar hooks/MCP.  
5. **Registrar** capacidades en el registro global (tools derivadas MCP, comandos, perfiles agente).

**Eco Go:** `internal/plugin` con `LoadAll(ctx, PluginLoadRequest) (*PluginBundle, error)` que devuelve contribuciones **mergidas** hacia `tools`, `mcp`, `agentprofile`, `hooks`, `skills` sin que cada paquete importe al orquestador circularmente — el **main** o `config` ensambla.

---

## 6. userConfig y variables

Opciones declaradas en manifiesto → prompt al habilitar → valores en `pluginConfigs` (no sensibles) o almacén de secretos (sensibles). Sustitución `${user_config.KEY}` y env `CLAUDE_PLUGIN_OPTION_*` para hooks/comandos.

**Eco Go:** `plugin.ConfigSchema` + validación; alineado con manejo de secretos en [CLAUDE.md](../../goclaw/CLAUDE.md) y [security.md](../goclaw/security.md).

---

## 7. Comando `/plugin` (referencia)

Instalar, gestionar, validar manifiesto, marketplaces. **Eco Go:** subcomandos `assistant plugin …` cuando exista producto; antes: solo `--plugin-dir` para desarrollo.

---

## 8. Estado vs ampliaciones posibles

| Alcance | Plugins |
|---------|---------|
| **goclaw hoy** | Sin marketplace remoto; **`--plugin-dir`**, `plugin_dirs` / allow y deny lists, manifiesto `goclaw-plugin.json`, merge de skills / hooks / agents según **D20** |
| **No implementado** | Marketplace npm/remoto, actualizaciones automáticas, dependencias entre plugins como en el producto de referencia |
| **Futuro (si se prioriza)** | Más auto-detect + paridad de manifiesto con §2; ver [roadmap.md](../goclaw/roadmap.md) optional waves |

---

## 9. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: 9 capacidades, layout, marketplace, política, eco Go, enlace [plugins (helmcode)](https://claude-code-explain.helmcode.com/plugins) |
