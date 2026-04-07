# Hooks (eventos → automatización) — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.12](ARCHITECTURE.md). Referencia (terceros, análisis de Claude Code): [Hooks — claude-code-explain](https://claude-code-explain.helmcode.com/hooks).

Los **hooks** son el pegamento **event-driven** del producto analizado: reaccionan a ciclo de vida de herramientas, sesión, permisos, compactación, MCP, worktrees, etc. Encajan **entre** el orquestador y el mundo exterior (shell, HTTP, LLM auxiliar, agente multi-turno).

---

## 1. Por qué importa (y dónde encaja en nuestros docs)

| Documento / capa | Relación |
|------------------|----------|
| [ARCHITECTURE.md §2.3–2.4](ARCHITECTURE.md) permisos y shell | `PreToolUse` / `PermissionRequest` pueden **bloquear**, **preguntar** o **mutar inputs** antes de ejecutar; integración explícita con reglas deny/ask/allow. |
| [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) | Clasificador **lateral**; los hooks son otra capa (p. ej. política local, formateo, CI). **D17** y **D18** deben fijar el **orden** respecto a reglas estáticas, fast paths y API del clasificador. |
| [CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md) | Eventos `PreCompact` / `PostCompact` para telemetría, backup o anuncios al usuario. |
| [MEMORY_SYSTEM.md](MEMORY_SYSTEM.md) / §2.6 | `InstructionsLoaded` cuando cargan reglas o índice de memoria. |
| [COORDINATOR_MODE.md](COORDINATOR_MODE.md) | `SubagentStart` / `SubagentStop`, tareas de equipo (`TaskCreated`, …). |
| §2.9 **Skills** | Frontmatter puede declarar hooks **en memoria** (sesión), lifecycle acotado. |
| [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) | Frontmatter **`hooks`**: mismos eventos; registro al **spawn** del sub-agente, limpieza al terminar (referencia). |
| [PLUGINS.md](PLUGINS.md) | `hooks/hooks.json` empaquetado; prioridad **plugin** en la cascada (§3); merge con MCP/agentes del mismo paquete (**D20**). |
| OpenClaw | [OPENCLAW_AGENTS_AND_TOOLS.md §8](OPENCLAW_AGENTS_AND_TOOLS.md) — carpeta `src/hooks/` como espejo. |

---

## 2. Modelo mental

- **~27 eventos** nombrados (lista agrupada en la **sección 4** de este documento).
- **4 tipos** de implementación: `command` (stdin JSON → stdout JSON/texto), `prompt` (evaluación LLM), `agent` (hasta N turnos con tools), `http` (POST cuerpo JSON; guardas SSRF en ref.).
- **7 fuentes** con **prioridad** (policy managed → usuario → proyecto → local → plugin → sesión en memoria → hooks programáticos). Para un mismo evento, **todos los hooks que casen se ejecutan en paralelo** (en referencia).
- **stdin:** payload JSON base (`session_id`, `cwd`, `hook_event_name`, `permission_mode`, `agent_id`, …) + campos por evento (`tool_name`, `tool_input`, …).
- **stdout:** si empieza por `{`, se parsea como JSON con campos comunes (`continue`, `decision`, `hookSpecificOutput`, …).
- **Exit code 2:** **bloquea** la operación y **stderr** puede mostrarse al modelo o usuario según evento (p. ej. `PreToolUse` bloquea la tool).

---

## 3. Fuentes (prioridad, referencia)

| # | Origen | Alcance típico |
|---|--------|----------------|
| 1 | Policy / managed | Enterprise, máxima autoridad |
| 2 | `~/.claude/settings.json` | Usuario, todos los proyectos |
| 3 | `.claude/settings.json` | Proyecto / equipo |
| 4 | `.claude/settings.local.json` | Local al proyecto |
| 5 | Plugins `hooks.json` | Alcance plugin |
| 6 | Sesión (skills/agents) | En RAM, temporal |
| 7 | Function hooks (SDK) | Programático |

**Eco Go:** cargar y fusionar desde `~/.config/assistant/hooks.yaml` (usuario) + opcional `.assistant/hooks` (proyecto) solo si **workspace trust** explícito; flags `AllowManagedHooksOnly`, `DisableHooks` equivalentes a la referencia.

---

## 4. Eventos (lista de trabajo; nombres alineados con referencia)

**Tool:** `PreToolUse`, `PostToolUse`, `PostToolUseFailure`.

**Sesión:** `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `Stop`, `StopFailure`, `Setup`.

**Permisos:** `PermissionRequest`, `PermissionDenied`, `Notification`.

**Subagentes / equipo:** `SubagentStart`, `SubagentStop`, `TeammateIdle`, `TaskCreated`, `TaskCompleted`.

**Contexto:** `PreCompact`, `PostCompact`, `InstructionsLoaded`, `ConfigChange`.

**MCP / filesystem:** `Elicitation`, `ElicitationResult`, `WorktreeCreate`, `WorktreeRemove`, `CwdChanged`, `FileChanged`.

---

## 5. Coincidencia: `matcher` e `if`

- **matcher:** exacto, lista `|`, o regex; según evento se compara con `tool_name`, `source`, etc.
- **`if`:** filtro secundario con sintaxis tipo reglas de permiso (p. ej. `Bash(git *)`, `Write(*.ts)`), evaluado **antes** de spawn (ahorra procesos).

---

## 6. Integración con permisos (PreToolUse)

En referencia, la salida del hook (`allow` / `deny` / `ask`) **no** pisa reglas **deny/ask** de configuración: **deny > ask > allow** entre hooks; además **reglas deny/ask del settings ganan** a un hook que dijera allow.

**Eco Go:** `permissions.Resolve()` debe recibir resultado agregado de hooks tras ejecutarlos en paralelo (o secuencial si simplificáis v1).

---

## 7. Async y `asyncRewake`

- `async: true`: no bloquea el bucle principal.
- Primera línea stdout puede declarar `{"async": true, "asyncTimeout": N}`.
- `asyncRewake`: salida con código 2 en background puede **despertar** de nuevo al modelo (p. ej. CI rojo) vía cola de notificaciones — patrón avanzado.

---

## 8. Seguridad (crítico)

> Los hooks `command` corren con **permisos del usuario**, sin sandbox obligatoria. Un `.claude/settings.json` malicioso en el repo puede ejecutar código arbitrario al abrir el proyecto.

**Mitigaciones de referencia:** `allowManagedHooksOnly`, `disableAllHooks` por capa; **workspace trust** antes de cargar hooks del proyecto; HTTP hooks con bloqueo localhost/red interna (SSRF).

**Eco Go:** por defecto **deshabilitar** hooks de proyecto hasta confirmación; auditar cada `command` en logs (ruta config, hash); timeouts estrictos; `SessionEnd` con tiempos cortos (en ref. ~1.5 s por defecto).

---

## 9. Eco Go (diseño sugerido)

| Pieza | Rol |
|-------|-----|
| `internal/hooks` | Registro por nombre de evento; carga config; `Fire(ctx, Event, payload) ([]HookResult, error)` |
| `hook.CommandRunner` | `exec` con timeout, env prefijado (`ASSISTANT_PROJECT_DIR`, …) |
| `hook.HTTPPoster` | Cliente HTTP con política SSRF |
| `hook.PromptRunner` / `hook.AgentRunner` | Reutilizar `llm` con presupuesto acotado |
| Orquestador | Emite eventos en puntos fijos: antes/después de tool, permiso, compact, etc. |

**Dependencias:** `hooks` puede importar `os/exec`, `net/http` acotado; **evitar** `hooks` → `orchestrator`. Quien orquesta llama a `hooks.Fire` y fusiona resultados con `permissions`.

---

## 10. Roadmap (alineado [ARCHITECTURE.md §4.4](ARCHITECTURE.md))

| Fase | Hooks |
|------|--------|
| MVP | Ninguno obligatorio; diseñar **interfaces** en `permissions` para dejar sitio |
| **v2** | Subconjunto: `PreToolUse` / `PostToolUse` + `command` + merge con permisos; proyecto **off** por defecto |
| **v3+Paridad** | Catálogo amplio de eventos; `prompt`/`agent`/`http`; skills + **agentes** frontmatter ([CUSTOM_AGENTS.md](CUSTOM_AGENTS.md)); `PreCompact` / `PermissionRequest` |

---

## 11. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: eventos, tipos, fuentes, permisos, async, seguridad, eco Go, enlace helmcode §18 |
| 2026-04-07 | §1 y roadmap v3: [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) (hooks en frontmatter de agente). |
