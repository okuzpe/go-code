# Reintentos y backoff (API LLM) — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.17](ARCHITECTURE.md) y al cliente `internal/llm`. Referencia (terceros): [Retry Logic — claude-code-explain](https://claude-code-explain.helmcode.com/retry-logic).

Objetivo: **absorber fallos transitorios** de red y de sobrecarga del proveedor sin tumbar la sesión; **no** enmascarar errores permanentes (auth, cuota real agotada tras `Retry-After`, payload inválido).

---

## 1. Flujo conceptual (referencia)

1. Fallo en la llamada a la API.
2. Clasificar **código HTTP** / error del SDK (transitorio vs terminal).
3. Calcular espera: **backoff exponencial** con techo.
4. Esperar (respetando `context.Context` para cancelación usuario).
5. Reintentar hasta **presupuesto** agotado → propagar error al orquestador / usuario.

**Por llamada, no global:** en referencia el contador de reintentos se **reinicia en cada invocación** HTTP al modelo. Diez `tool_use` en una sesión implican **diez presupuestos independientes** de reintentos para las Completion correspondientes; un stream lento no “gasta” el cupo de otra llamada.

**Eco Go:** implementar el bucle de reintento **dentro** de `llm.Complete` / `llm.Stream` (o un transporte interno), no como estado global en `orchestrator`.

---

## 2. Parámetros de referencia (Claude Code analizado)

| Parámetro | Valor orientativo | Notas |
|-----------|-------------------|--------|
| Reintentos máx. (defecto) | **10** | Por llamada |
| Backoff base | **500 ms** | Duplica cada intento hasta el techo |
| Backoff máximo | **5 min** | Tope por espera entre intentos |
| **429** Rate limit | Hasta 10 | Preferir cabecera **`Retry-After`** si existe |
| **529** Overloaded | **3** | En referencia: solo **foreground**; **background** no reintenta 529 |
| Otros **5xx** | Hasta 10 | Backoff exponencial estándar |
| **401** / **403** / 4xx de cliente | **No** retry típico | Corregir credenciales o permisos |

Los números exactos son del producto analizado; **D22** fija los vuestros por proveedor (**D1**, **D10**).

---

## 3. Modo “unattended” (referencia, interno)

Para tareas largas automatizadas (no sesión interactiva estándar), la referencia describe:

- Reintentos **sin límite** fijo de intentos (con cuidado).
- **Heartbeat** ~30 s para mantener el proceso vivo.
- **Tope duro ~6 h** para evitar procesos colgados.
- No expuesto como modo general del usuario.

**Eco Go:** si algún día hay `assistant daemon` o CI, considerar un **modo explícito** con flag + el mismo tope de tiempo; nunca por defecto en REPL.

---

## 4. Interacción con otras capas

| Capa | Relación |
|------|----------|
| [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) | El clasificador lateral también llama al LLM: aplicar política de reintentos **acotada**; en fallo repetido, *fail closed* (**D17**), no bucle infinito. |
| [CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md) | El agente de compactación es otra llamada al modelo: mismo cliente; timeouts separados del turno principal. |
| [LOCAL_MODELS.md](LOCAL_MODELS.md) | Ollama/local suele fallar con otros patrones (conexión rechazada, OOM); conviene tabla de errores **local vs cloud**. |
| `web_fetch` / tools | Reintentos de **tool** son decisión distinta (p. ej. 1 retry en 5xx); no mezclar con la política del **LLM**. |

---

## 5. Eco Go (implementación sugerida)

| Pieza | Ubicación |
|-------|-----------|
| Estrategia por código de error | `internal/llm/retry.go` o paquete auxiliar |
| Backoff con jitter | Opcional: reduce thundering herd si varios clientes |
| `Retry-After` | Parsear según RFC; clamp al máximo configurado |
| Observabilidad | `slog` con intento N/M, duración de espera, código HTTP (sin body de secretos) |
| Config | Flags o env: `ASSISTANT_LLM_MAX_RETRIES`, `ASSISTANT_LLM_RETRY_BASE_MS`, … (**D22**) |

**MVP:** al menos **1–2 reintentos** en timeouts de red o 502/503; **v1+** alinear con tabla §2 y proveedor real.

---

## 6. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: flujo, códigos, unattended, capas Go, **D22** |
