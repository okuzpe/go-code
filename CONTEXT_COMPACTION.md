# Contexto y compactación — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.5](ARCHITECTURE.md). Referencia conceptual (terceros, análisis del código de Claude Code): [Context & Compaction — claude-code-explain](https://claude-code-explain.helmcode.com/context-compaction).

Los números concretos de esta página son **del producto analizado**, no compromisos de nuestro binario: sirven para **calibrar** implementación y flags de configuración (ver [ARCHITECTURE.md §5 D15](ARCHITECTURE.md)).

---

## 1. Por qué encaja en nuestros documentos

| Pieza ya en ARCHITECTURE | Relación |
|--------------------------|----------|
| §2.2 “límites de contexto” en el cliente LLM | La compactación es política **alrededor** de ese cliente + el historial en `session`. |
| §2.5 (este tema) | Micro-compactación vs compactación fuerte; reinyección opcional. |
| §2.7 perfiles | Menos contexto inyectado ⇒ menos presión sobre la ventana; no sustituye podar **tool results**. |
| §2.10 memoria | Tras un resumen agresivo, `MEMORY.md` y ficheros de memoria siguen siendo la capa **estable** fuera del hilo. |
| §4.3 `session` → `llm` evitar | Quien compacta con **otra llamada al modelo** suele ser `orchestrator` o un worker, no el paquete de estado puro. |
| §2.17 / [RETRY_LOGIC.md](RETRY_LOGIC.md) | Esa llamada extra hereda la misma política de **reintentos/backoff** (**D22**), con cancelación acotada. |

---

## 2. Ventana de contexto (referencia analizada)

| Contexto | Tokens (ref.) | Nota |
|----------|---------------|------|
| Por defecto | ~200 000 | “Default” en el producto de referencia |
| Modelos “1M” | ~1 000 000 | Ej.: familias Sonnet 4.x / Opus 4.6 con modo ampliado |
| Anulación | Personalizado | En referencia: p. ej. variable tipo `CLAUDE_CODE_MAX_CONTEXT_TOKENS` |

**Eco Go:** leer límite efectivo del proveedor (API) o del runtime local (Ollama: contexto del modelo cargado, mucho menor que 200K en práctica — ver [LOCAL_MODELS.md](LOCAL_MODELS.md)) y expon optionally override en config.

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

- **Compactación** ≠ **memoria persistente** ([MEMORY_SYSTEM.md](MEMORY_SYSTEM.md)): el resumen vive en el hilo; la memoria en disco evita perder hechos estables cuando el hilo se resume.
- **Micro-compactación** ≠ **truncar logs en disco**: solo el **mensaje en vuelo** hacia el API.
- Números 13K / 50K / 32K son **referencia**: con Ollama 7B el techo real puede ser 8K–32K según modelo; los umbrales deben ser **fracción del límite configurado** o tablas por proveedor (D15).
- **Multi-agente:** el hilo del **coordinador** y el de cada **worker** son independientes; compactar uno no reemplaza el historial de los demás — ver [COORDINATOR_MODE.md §7](COORDINATOR_MODE.md).

---

## 7. Roadmap sugerido (alineado con ARCHITECTURE §4.4)

| Fase | Compactación |
|------|----------------|
| MVP | Techo simple de historial (p. ej. N mensajes o bytes); opcional poda naive de tool results |
| v1 | Micro-compactación con reglas de edad + estimación de tokens; umbral proporcional al modelo |
| v2+ | Auto-compact con sub-llamada + cortacircuitos + reinyección acotada + comando manual |

---

## 8. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: ventana, auto/micro-compact, límites de salida, eco Go, enlace helmcode §06 |
| 2026-04-07 | §6: multi-agente (hilos separados) → [COORDINATOR_MODE.md](COORDINATOR_MODE.md). |
| 2026-04-07 | §1: tabla + [RETRY_LOGIC.md](RETRY_LOGIC.md) (sub-llamada compactación). |
