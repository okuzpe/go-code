# Contexto y compactación — referencia y eco Go

**Status in goclaw:** Session compaction uses a **token heuristic** and configurable threshold — [`goclaw/internal/orchestrator/compaction.go`](../../goclaw/internal/orchestrator/compaction.go), **D15** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). Numbers below still calibrate against the **reference product**, not hard limits in Go.

Profundidad ligada a [CLAUDE.md](../../goclaw/CLAUDE.md) (D15 compaction). Referencia conceptual (terceros, análisis del código de Claude Code): [Context & Compaction — claude-code-explain](https://claude-code-explain.helmcode.com/context-compaction).

Los números concretos de esta página son **del producto analizado**, no compromisos de nuestro binario: sirven para **calibrar** implementación y flags de configuración (ver [CLAUDE.md](../../goclaw/CLAUDE.md) (D15)).

---

## 1. Por qué encaja en nuestros documentos

| Tema (ver [CLAUDE.md](../../goclaw/CLAUDE.md)) | Relación |
|-----------------------------------------------|----------|
| Límites de contexto / cliente LLM (**D15**) | La compactación es política **alrededor** de ese cliente + el historial en `session`. |
| Micro vs compactación fuerte (este doc §3–§4) | Micro-compactación vs compactación fuerte; reinyección opcional. |
| Perfiles de agente | Menos contexto inyectado ⇒ menos presión sobre la ventana; no sustituye podar **tool results**. |
| Memoria en disco (**D13**) | Tras un resumen agresivo, `MEMORY.md` y ficheros de memoria siguen siendo la capa **estable** fuera del hilo. |
| `session` vs `llm` | Quien compacta con **otra llamada al modelo** suele ser `orchestrator` o un worker, no el paquete de estado puro. |
| Reintentos / [retry-logic.md](./retry-logic.md) (**D22**) | Esa llamada extra hereda la misma política de **reintentos/backoff**, con cancelación acotada. |

---

## 2. Ventana de contexto (referencia analizada)

| Contexto | Tokens (ref.) | Nota |
|----------|---------------|------|
| Por defecto | ~200 000 | “Default” en el producto de referencia |
| Modelos “1M” | ~1 000 000 | Ej.: familias Sonnet 4.x / Opus 4.6 con modo ampliado |
| Anulación | Personalizado | En referencia: p. ej. variable tipo `CLAUDE_CODE_MAX_CONTEXT_TOKENS` |

**Eco Go:** leer límite efectivo del proveedor (API) o del runtime local (Ollama: contexto del modelo cargado, mucho menor que 200K en práctica — ver [local-models.md](./local-models.md)) y expon optionally override en config.

---

## 3. Auto-compactación (compactación fuerte)

Flujo descrito en la referencia:

1. **Umbral:** quedan ~**13 000** tokens libres antes del techo → se dispara compactación automática.
2. **Sub-agente / fork:** un proceso lee el historial completo y genera un **resumen comprimido**.
3. **Sustitución:** el hilo que ve el modelo principal pasa a ser ese resumen (no el historial íntegro).
4. **Presupuesto post-compact:** ~**50 000** tokens para “recuperar” contexto útil; en referencia hay límites tipo **máx. 5 archivos**, **~5 000 tokens por archivo** (p. ej. skills y ficheros clave).
5. **Cortacircuito:** si la compactación **falla 3 veces seguidas**, deja de reintentar para evitar bucles.

**Eco Go:** tareas asíncronas con `context.Context`, mismo cliente `llm` pero system prompt de “summarizer”, métrica de fallos consecutivos, y política de reinyección explícita (leer de disco bajo presupuesto en lugar de confiar en que el modelo “recuerde” el repo).

**Compactación manual:** en referencia existe un comando tipo **`/compact`** antes de llegar al límite; suele producir resúmenes más intencionados que un auto-compact de último minuto. Equivalente Go: comando REPL o flag que invoque el mismo pipeline con confirmación.

---

## 4. Micro-compactación (inline, durante la sesión)

Objetivo: **no** esperar al umbral global; reduce ruido de **resultados de herramientas antiguos**.

- Cuando un `tool_result` “envejece” (heurística: antigüedad en el historial o turnos desde el último uso), su contenido se reemplaza por un marcador corto, p. ej. **`[Old tool result content cleared]`** (texto ilustrativo del patrón analizado).
- Herramientas cubiertas en la referencia para este tratamiento incluyen entre otras: **Read, Bash, Grep, Glob, WebSearch, WebFetch, Edit, Write**.

**Imágenes:** en referencia se estima un coste fijo conservador (**~2 000 tokens** por imagen) para el presupuesto aunque el fichero sea distinto.

**Eco Go:** implementación en `internal/session` o `internal/contextwindow`: cola de mensajes con metadatos por bloque (`ToolName`, `tokensEstimate`, edad); función `MicroCompact(messages)` antes de cada `Complete`. Cuidado con **no** borrar resultados que el último turno del usuario aún necesita (heurística o “pinning” explícito en v2).

---

## 5. Límites de salida del modelo (referencia)

Constantes orientativas del producto analizado (útiles al dimensionar `max_tokens` por fase):

| Modo | Tokens máx. salida (ref.) |
|------|---------------------------|
| Respuesta estándar | ~32 000 |
| Capado (reserva de “slots”) | ~8 000 |
| Escalado / recuperación tras errores | ~64 000 |
| Salida del **agente de compactación** (resumen) | ~20 000 |

**Eco Go:** parametrizar por tipo de llamada (`TurnUser`, `TurnCompaction`, `TurnRecovery`) en `internal/llm`.

---

## 6. Qué no mezclar

- **Compactación** ≠ **memoria persistente** ([memory-system.md](./memory-system.md)): el resumen vive en el hilo; la memoria en disco evita perder hechos estables cuando el hilo se resume.
- **Micro-compactación** ≠ **truncar logs en disco**: solo el **mensaje en vuelo** hacia el API.
- Números 13K / 50K / 32K son **referencia**: con Ollama 7B el techo real puede ser 8K–32K según modelo; los umbrales deben ser **fracción del límite configurado** o tablas por proveedor (D15).
- **Multi-agente:** el hilo del **coordinador** y el de cada **worker** son independientes; compactar uno no reemplaza el historial de los demás — ver [coordinator-mode.md §2.7](./coordinator-mode.md) (aislamiento del worker).

---

## 7. goclaw hoy vs mejoras opcionales

| Ámbito | Compactación |
|------|----------------|
| **goclaw (shipped)** | Heurística de tokens + umbral configurable, **tail** de turnos recientes preservado, fase 1 que limpia `tool_results` antiguos, `/compact`, opción **`llm_compaction`** + **`compaction_model`** — ver [`compaction.go`](../../goclaw/internal/orchestrator/compaction.go) y **D15** en [`CLAUDE.md`](../../goclaw/CLAUDE.md) |
| **Referencia** | Micro-compact agresiva, presupuestos post-compact de decenas de miles de tokens, etc. — calibración, no exigencia |
| **No implementado / más adelante** | Reinyección automática de ficheros bajo presupuesto como en algunos productos referencia; ver [roadmap.md](../goclaw/roadmap.md) si se prioriza |

---

## 8. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: ventana, auto/micro-compact, límites de salida, eco Go, enlace helmcode §06 |
| 2026-04-07 | §6: multi-agente (hilos separados) → [coordinator-mode.md](./coordinator-mode.md). |
| 2026-04-07 | §1: tabla + [retry-logic.md](./retry-logic.md) (sub-llamada compactación). |
