# Memoria persistente entre sesiones — referencia y eco Go

**Status in goclaw:** Filesystem store under `~/.goclaw/memory/` with `MEMORY.md` index; REPL `/memory` — see **D13** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). This doc adds **reference-product** depth.

Profundidad ligada a [CLAUDE.md](../../goclaw/CLAUDE.md) (D13 memory). Referencia conceptual (terceros): [Memory — claude-code-explain](https://claude-code-explain.helmcode.com/memory).

---

## 1. ¿Vale la pena incluirlo?

**Sí**, y **ya está en goclaw** (D13): el historial en RAM (`internal/session`) no sobrevive al cierre del proceso; la memoria en disco captura **hechos estables sobre usuario, proyecto y feedback** sin rellenar el prompt con git ni código duplicado. El resto del doc profundiza el patrón del **producto de referencia** para calibrar límites y UX.

**No es lo mismo que**

| Concepto | Rol |
|----------|-----|
| `session` | Turnos de la conversación **actual**; compactación / [context-compaction.md](./context-compaction.md) |
| `AGENTS.md` / `CLAUDE.md` | Convenciones del repo, versionadas con el código |
| **Memory** (este doc) | Hechos **opacos** al código: rol del usuario, correcciones validadas, plazos, punteros a Linear/Slack, etc. |
| Memoria **por agente** (`memory: user/project/local` en frontmatter) | Directorios dedicados por agente + índice tipo `MEMORY.md`; distinto alcance que el índice global — [custom-agents.md §5](./custom-agents.md). |

---

## 2. Cuatro tipos (taxonomía de referencia)

| Tipo | Contenido típico | Cuándo guardar (heurística) |
|------|------------------|----------------------------|
| **user** | Rol, preferencias, nivel (ej. “data scientist, nuevo en React”) | Al aprender estilo de trabajo o metas del usuario |
| **feedback** | Correcciones y enfoques **validados** (ej. “no mockear la DB en tests”) | Tras corrección explícita o confirmación de que algo funcionó |
| **project** | Trabajo en curso, fechas, incidentes (ej. “merge freeze 2026-03-05”) | Al clarificar quién, qué, cuándo; convertir fechas relativas a absolutas |
| **reference** | Punteros a sistemas externos (ej. “bugs de pipeline en Linear INGEST”) | Al citar herramientas/canales/proyectos externos |

Cada entrada puede ser un **Markdown** con **frontmatter YAML** (tipo, fecha, título); el índice agrega referencias.

---

## 3. Índice `MEMORY.md` y límites duros

En el producto de referencia el índice:

- Va siempre cargado en contexto (capa dinámica del system prompt).
- Tiene **tope ~200 líneas** y **~25 KB**; si se supera, se **trunca** (en referencia, sin aviso explícito al usuario).

**Tip de producto (referencia):** [practical-tips.md §8](./practical-tips.md) — reforzar que el índice sea **solo punteros** y valorar **avisar** antes del truncado.

**Eco diseño**

- Implementar los mismos límites con **medición en bytes y líneas** antes de inyectar en el prompt.
- Mejora de UX posible: **avisar** (log o TUI) cuando el índice supere umbral y debas condensar manualmente.
- Entradas del índice **cortas** (p. ej. meta de ~150 caracteres por fichero hijo), una línea por fichero.

---

## 4. Estructura de ficheros (propuesta para nuestro CLI)

No estamos obligados a `~/.claude/`; conviene un namespace propio, p. ej.:

```
~/.config/assistant/projects/<slug>/memory/
├── MEMORY.md              # índice (límites como §3)
├── user_role.md
├── feedback_testing.md
└── reference_tools.md
```

O por proyecto en el repo: `.assistant/memory/` (decisión **D7** / **D14**). El **slug** puede ser hash del path del repo o nombre configurado.

---

## 5. Qué **no** debe ir en memoria

Evitar contaminación y staleness:

- Patrones de código o rutas deducibles del repo (eso va en docs del proyecto o el propio código).
- Historia git o autores (usar `git log` / `git blame`).
- “Recetas” de debug cuyo resultado ya está en el código.
- Lo que ya vive en `AGENTS.md` / `CLAUDE.md` (duplicar divide la verdad).
- Detalles efímeros de la sesión actual.
- Listas de PRs o resúmenes de actividad (envejecen mal).

---

## 6. Extracción automática (fase avanzada)

Patrón del producto de referencia (resumen):

| Aspecto | Comportamiento |
|---------|----------------|
| Ejecución | **Background**: fork / sub-agente sin bloquear la sesión principal |
| Disparo | Tras turnos **sin** llamadas a herramientas (“silent turns”) |
| Presupuesto | Máx. **~5 turnos** del agente extractor |
| Permisos | Read/Grep/Glob amplios; Bash **solo lectura**; Write/Edit **solo** bajo `memory/` |
| Dedup | Si el agente principal ya escribió memoria en la sesión, no repetir extracción |
| Caché | Compartir caché de prompt con la sesión padre cuando el proveedor lo permita |

**Eco Go:** goroutine + `context`, perfil tipo **Explore** + escritura acotada al directorio memoria; política de disparo configurable.

---

## 7. Encaje en paquetes Go

| Pieza | Paquete |
|-------|---------|
| Cargar índice + fragmentos para el prompt | `internal/memory` + llamada desde `internal/prompt` o `orchestrator` |
| Herramientas `memory_read` / `memory_write` (opcional) | `internal/tools`, con **permissions** que limiten ruta al árbol memoria |
| Extractor en segundo plano | `internal/orchestrator` o `internal/memory/extractor.go` |

Dependencias: `orchestrator` → `memory`; `memory` **no** importa `orchestrator`.

---

## 8. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación a partir del modelo Claude Code (explicador tercero); límites, 4 tipos, anti-patrones, extractor, rutas propias. |
| 2026-04-07 | Enlace explícito a [context-compaction.md](./context-compaction.md) en la tabla §1 (`session`). |
| 2026-04-07 | §1: memoria por agente vs índice global → [custom-agents.md](./custom-agents.md) §5. |
| 2026-04-07 | §3: enlace [practical-tips.md §8](./practical-tips.md) (truncado índice). |
