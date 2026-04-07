# Perfiles de agente (tipo Claude Code) — referencia y eco Go

Documento de profundidad alineado con [ARCHITECTURE.md §2.7](ARCHITECTURE.md). Fuente conceptual (terceros): [Claude Code internals — Agents](https://claude-code-explain.helmcode.com/agents). **Extensiones en Markdown** (mismo contenido conceptual en frontmatter + cuerpo = system prompt): [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) y §2.13 en ARCHITECTURE.

---

## 1. Idea central

Un **agente** no es solo “el modelo”: es un **perfil** reproducible con:

- **Modelo** (fijo, heredado del padre, o barato tipo “Haiku” para tareas acotadas).
- **Conjunto de herramientas** (allowlist / bloqueos explícitos).
- **Modo de permisos** (p. ej. interactivo vs `dontAsk`).
- **Política de contexto** (qué se carga en el system prompt: reglas del proyecto, `git status`, etc.).

Los sub-agentes se **delegan** con ese perfil; el orquestador no mezcla herramientas peligrosas en un explorador solo lectura.

---

## 2. Los 6 tipos integrados (referencia de producto)

| ID | Tipo | Modelo (ref.) | Herramientas (resumen) | Modo permisos / notas |
|----|------|---------------|------------------------|------------------------|
| 01 | **General-Purpose** | Hereda del padre | Todas (`*`) | Estándar; comodín cuando no aplica otro perfil |
| 02 | **Explore** | Haiku (ext.) / hereda (int.) | Todas **excepto** Agent, Edit, Write, ExitPlanMode, NotebookEdit | Solo lectura en disco; **no** sub-agentes |
| 03 | **Plan** | Hereda del padre | Solo lectura (sin Edit, Write, Agent, ExitPlanMode) | Salida esperada: **3–5 archivos críticos** para implementar |
| 04 | **Verification** | Hereda del padre | Solo herramientas de verificación (lectura / tests) | **Background**; debe terminar con `VERDICT: PASS|FAIL|PARTIAL` |
| 05 | **Claude Code Guide** | Haiku | Glob, Grep, Read, WebFetch, WebSearch | **`dontAsk`** (niega prompts automáticamente); solo consultas de documentación |
| 06 | **Status Line Setup** | Sonnet | **Solo** Read + Edit | Sin shell ni web; propósito único (config de status line) |

---

## 3. Lección de tokens: omitir reglas pesadas en sub-agentes

En el producto de referencia, **Explore** y **Plan** **no** cargan `CLAUDE.md` ni `git status` en contexto, para ahorrar tokens a escala (millones de spawns/semana).

**Eco nuestro:**

- Definir en cada perfil `ContextAttachments`: p. ej. `ProjectRules`, `GitStatus`, `SkillsIndex`, … con flags `true/false`.
- Para perfiles “baratos” de sólo lectura o planificación, desactivar adjuntos grandes y **inyectar solo** lo imprescindible en el prompt de la tarea (o citar rutas puntuales).
- Si necesitáis convenciones del repo en Explore/Plan, **pegarlas en el mensaje de delegación** en lugar de re-abrir todo el fichero de reglas en cada spawn.

---

## 4. Encaje en Go (propuesta)

| Concepto | Dónde vive |
|----------|------------|
| Definición de perfil (nombre, model override, tool filter, permission mode, context policy) | `internal/agentprofile` o `internal/orchestrator/profile.go` |
| Construcción del `ToolRegistry` **vista** por este turno | `tools.Registry.View(Profile)` o lista filtrada antes de llamar al LLM |
| Bucle de sub-agente | Misma pieza que `internal/orchestrator`, con `Run(ctx, Profile, SessionFork)` |
| Verificación en background | Goroutine + canal de resultado; UI (más adelante) distingue “job rojo” |

**MVP:** un único perfil equivalente a **General-Purpose**.  
**v1/v2:** añadir **Explore** y **Plan** (ahorro de contexto + menos permisos).  
**v3:** **Verification** en CI; perfiles tipo **Guide** / **Status Line** solo si hay producto equivalente.

---

## 5. Relación con otras decisiones

- **D12 (herramientas dedicadas):** todos los perfiles deben seguir §2.1; un Explore con Bash para `cat` sería incoherente.
- **D1–D11 (LLM local):** perfiles “Haiku barato” se mapean a **otro id de modelo** en Ollama o a “mismo modelo, max_tokens menor”; no hay magia: hay que fijar política explícita. Tabla orientativa **perfil → modelo / VRAM** y notas sobre **LM Studio** vs **Ollama:** [LOCAL_MODELS.md §2.5–§2.7](LOCAL_MODELS.md).
- **Memoria persistente ([MEMORY_SYSTEM.md](MEMORY_SYSTEM.md)):** un agente **extractor** de memoria debería usar perfil **Explore**-like (solo lectura en repo) y escritura **acotada** al directorio de memoria.
- **D16 / multi-agente ([COORDINATOR_MODE.md](COORDINATOR_MODE.md)):** el rol **Coordinator** (hub-and-spoke) no es un séptimo “tipo” integrado de la tabla §2: es un **perfil de herramientas** del centro — allowlist de orquestación y **cero** Read/Write/Bash. Los **workers** sí usan perfiles tipo General-Purpose o especializados con escritura. Cada delegación a worker debe repetir todo el contexto útil (§3 y regla “no ven al coordinador” en [COORDINATOR_MODE.md §2.7](COORDINATOR_MODE.md)).
- **D17 / auto-modo ([YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md)):** un perfil con **`dontAsk`** o batch sin prompts solo es razonable si el **gate previo** (fast paths + clasificador lateral + política de proyecto **no** inyectada desde repo sin validación) está activo; de lo contrario el modelo puede encadenar herramientas peligrosas sin freno.
- **D19 / agentes `.md` ([CUSTOM_AGENTS.md](CUSTOM_AGENTS.md)):** los seis tipos de §2 siguen siendo la base **built-in**; los ficheros custom **override por nombre** en referencia — conviene documentar en equipo qué `name` usáis para no chocar con Explore/Plan.

---

## 6. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: tabla 6 tipos, contexto omitido, eco Go, fases. |
| 2026-04-07 | §5: enlace a agente extractor de memoria. |
| 2026-04-07 | §5: D16, Coordinator vs perfiles §2, enlace [COORDINATOR_MODE.md](COORDINATOR_MODE.md). |
| 2026-04-07 | §5: D17, `dontAsk` vs [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md). |
| 2026-04-07 | Intro: enlace a agentes personalizados [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md). |
| 2026-04-07 | §5: D19, override por nombre vs §2. |
