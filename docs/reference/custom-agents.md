# Agentes personalizados (Markdown + frontmatter) — referencia y eco Go

**Status in goclaw:** **D19 implemented** — Markdown + YAML frontmatter in `~/.goclaw/agents/*.md` and `.goclaw/agents/*.md`; see [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md) and [`agent-profiles.md`](./agent-profiles.md).

Profundidad ligada a [CLAUDE.md](../../goclaw/CLAUDE.md) (D19 custom agents) y [agent-profiles.md](./agent-profiles.md). Referencia (terceros, análisis de Claude Code): [Custom Agents — claude-code-explain](https://claude-code-explain.helmcode.com/custom-agents).

**Idea:** un **.md por agente** con **YAML frontmatter** que fija identidad operativa (tools, modelo, permisos, MCP, hooks, memoria, color); el **cuerpo Markdown** es el system prompt. Sin código extra para “registrar” el agente más allá de colocarlo en una ruta de descubrimiento.

---

## 1. Dónde encaja en nuestros documentos

| Documento | Relación |
|-----------|----------|
| [agent-profiles.md](./agent-profiles.md) §2 | Los **7 perfiles integrados** (incl. `coordinator`) son el set **built-in**; un custom con el **mismo nombre** puede **sustituir** al built-in en referencia (prioridad). |
| [coordinator-mode.md](./coordinator-mode.md) | La tool **Agent** elige `subagent_type` → resuelve definición custom o built-in. |
| [hooks.md](./hooks.md) | Frontmatter **`hooks`**: registra hooks de **sesión** al spawn del sub-agente; se limpian al terminar; `Stop` → `SubagentStop` en referencia. |
| [memory-system.md](./memory-system.md) | Memoria de **proyecto** (`MEMORY.md`) ≠ memoria **por agente** (`memory: user|project|local` + directorio dedicado); ver §5 de este doc. |
| §2.8 **MCP** | `mcpServers` en frontmatter: referencias con nombre o definición inline; limpieza al finalizar agente si aplica. |
| §2.9 **Skills** | Campo `skills` para precargar contenido antes del primer turno. |
| [yolo-classifier.md](./yolo-classifier.md) | `permissionMode` del agente limita o expande riesgo; sigue pasando por **D17** en auto-modo. |

---

## 2. Tipos de definición y prioridad (referencia)

| Tipo | Origen típico |
|------|----------------|
| **Built-in** | Código (dinámico) — tabla en [agent-profiles.md §2](./agent-profiles.md) |
| **Custom** | `agents/*.md` en rutas de usuario / proyecto |
| **Plugin** | `plugin/agents/*.md` + restricciones de seguridad (§7) |
| **Flag CLI** | `--agents` JSON, solo sesión |

**Orden de prioridad** (más alto gana en referencia): managed enterprise → flag sesión → **proyecto** `agents/` → **usuario** `~/…/agents/` → **plugin** → **built-in** (más bajo).

**Eco Go:** tabla explícita en `internal/agentprofile` (`Resolve(name string, sources ...)`); flag env tipo `ASSISTANT_SIMPLE=true` puede **omitir** customs (equivalente `CLAUDE_CODE_SIMPLE`).

---

## 3. Formato del fichero (conceptual)

- **Rutas ilustrativas (ref.):** `<cwd>/.claude/agents/foo.md`, `~/.claude/agents/foo.md`.
- **Eco Go:** `.assistant/agents/*.md` y `~/.config/assistant/agents/*.md` (nombres exactos **D19** + D7).

**Frontmatter — campos clave**

| Campo | Rol |
|-------|-----|
| `name` | Identificador → `subagent_type` |
| `description` | **Crítico para selección:** “Use when…” concret; multilínea con `\n` |
| `tools` / `disallowedTools` | Allowlist / denylist (ver §4) |
| `model`, `effort` | Override o `inherit` |
| `permissionMode` | default, acceptEdits, bypassPermissions, dontAsk, plan, auto |
| `color` | Identidad UI (paletas fijadas en ref.) |
| `maxTurns`, `background` | Límites y ejecución en segundo plano |
| `memory` | `user` \| `project` \| `local` — ver §5 |
| `isolation` | p. ej. `worktree` (git aislado, limpieza auto) |
| `hooks` | Mismo esquema que [hooks.md](./hooks.md); alcance sesión del agente |
| `mcpServers` | Referencias o inline HTTP/stdio |
| `skills` | Nombres a precargar |

**Cuerpo:** system prompt tras el segundo `---`; si vacío → prompt genérico por defecto (en referencia).

---

## 4. Resolución de herramientas (referencia)

1. `tools` ausente → conjunto completo.  
2. `tools: []` → ninguna.  
3. `tools: ["*"]` → todas.  
4. Lista explícita → solo esas.  
5. `disallowedTools` encima de la allowlist.  
6. Si `memory` activo + lista explícita → en ref. se **inyectan** Read/Write/Edit para gestionar memoria del agente.  
7. Herramientas MCP del agente se **fusionan**.  
8. Agentes async pueden quitar tools tipo UserInput.

**Eco Go:** `agentprofile.ApplyToRegistry(base Registry) (*Registry, error)` coherente con `internal/tools`.

---

## 5. Memoria por agente vs MEMORY.md (§2.10)

Tres **ámbitos** en referencia (directorios bajo `.claude/` en el análisis):

| Scope | Ubicación típica | Notas |
|-------|------------------|--------|
| `user` | `~/.claude/agent-memory/<name>/` | Sin VCS obligatorio |
| `project` | `<cwd>/.claude/agent-memory/<name>/` | Puede versionarse |
| `local` | `…/agent-memory-local/<name>/` | Sin VCS |

Flujo: crear dir si falta → cargar índice tipo `MEMORY.md` → añadir directrices de scope al prompt.

**Snapshots:** equipo puede commitear baseline en `agent-memory-snapshots/` para hidratar agentes nuevos.

**Eco Go:** reutilizar paquete `memory/` con **prefijo de ruta** por agente y scope; no mezclar con el índice global del usuario en §2.10 sin decisión explícita (**D19**).

---

## 6. Invocación (tool Agent)

Campos conceptuales: `description`, `prompt`, `subagent_type`, `model`, `run_in_background`, `name`, `team_name`, `mode`, `isolation`, `cwd` (contextos avanzados).

**Fork vs fresh (referencia):** sin `subagent_type` puede heredar contexto padre; con tipo definido → prompt/tools propios y ventana fresca.

---

## 7. Agentes en plugins (restricciones, referencia)

Panorama del empaquetado: [plugins.md](./plugins.md). En modo restrictivo del producto analizado, agentes definidos por plugin **no** pueden: escalar `permissionMode`, registrar **hooks** custom arbitrarios, declarar **mcpServers** propios. Nombre con namespace `plugin:agent`.

**Eco Go:** `TrustLevel` del plugin + validación en loader.

---

## 8. No está en `settings.json` (referencia)

Los agentes **no** se declaran dentro del schema de settings; son ficheros o JSON `--agents`.

---

## 9. `/agents` (producto completo)

UI interactiva: listar por fuente, ver detalle, crear asistente (wizard), editar/borrar user/project. **Eco Go:** CLI `assistant agents list|show` en fase tardía; MVP puede leer solo disco.

---

## 10. Eco Go (resumen)

| Pieza | Ubicación sugerida |
|-------|---------------------|
| Descubrimiento y merge prioridades | `internal/agentprofile` (`discover.go`, `resolve.go`) |
| Tipo `CustomAgentDef` | Parse frontmatter + cuerpo; tests con `testdata/agents/*.md` |
| Construcción de system prompt | `internal/prompt` capas: cuerpo → memoria agente → env/CWD → AGENTS.md opcional |
| Hooks por agente | Delegado en `internal/hooks` con scope `agentID` |
| Worktree isolation | `internal/tools/git` o wrapper; flag **D19** |

**Roadmap:** v1 `agentprofile` solo built-ins + 1 custom path opcional; **v3+** paridad con prioridades, plugin, MCP/hooks en frontmatter — [roadmap.md](../goclaw/roadmap.md).

---

## 11. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: tipos, prioridad, frontmatter, tools, memoria agente, MCP/hooks, plugins, eco Go, enlace helmcode §20 |
