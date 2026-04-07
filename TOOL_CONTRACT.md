# Contrato de herramientas — MVP (v1 del documento)

**Propósito:** nombres estables, riesgo y límites de salida para `internal/tools` antes de implementar. Alineado con [ARCHITECTURE.md](ARCHITECTURE.md) §2.1, §3.1, §4.4 y decisión **D12**. Los esquemas JSON exactos se afinan en código; aquí la **tabla canónica** y política de red.

**Hub:** [ARCHITECTURE.md](ARCHITECTURE.md) §5, §5.1 (cierre pre-implementación).

---

## 1. Tabla MVP (fila §4.4)

| Nombre expuesto al modelo | Riesgo | Input (resumen) | Límite salida (orientativo) | Notas |
|---------------------------|--------|-------------------|-----------------------------|--------|
| `read_file` | `read_only` | `path` (relativo al workspace o allowlist), opcional `offset`/`limit` líneas | **512 KiB** o **200 líneas** por defecto (el menor que aplique); error si se excede sin chunking | Sin seguir symlinks fuera del workspace |
| `bash` / `run_terminal` | `shell` | `command` string, opcional `cwd` acotado | **stdout+stderr 256 KiB** truncado con aviso | **D4:** shell real según OS; MVP: confirmación usuario salvo allowlist mínima |
| `web_search` | `network` | `query` string | **Top N** resultados (N≤8), snippet **≤2 KiB** c/u | **D3:** si no hay proveedor configurado, la herramienta devuelve error claro “search not configured” |
| `web_fetch` | `network` | `url` HTTPS (u esquema permitido), opcional `max_bytes` | **1 MiB** por defecto; solo texto/HTML; sin ejecutar JS | Ver §2 política SSRF |

Nombres internos Go pueden diferir (`Read`, `Bash`); el **contrato al LLM** debe usar la columna “Nombre expuesto”.

**v1 posterior (no MVP):** `glob`, `grep` dedicados — [ARCHITECTURE.md](ARCHITECTURE.md) §2.1.

---

## 2. Política de red MVP (`web_fetch` y futuro `web_search`)

Inspiración de criterios de productos tipo OpenClaw upstream (tests SSRF en `web-fetch`). Implementación mínima antes de beta:

- **Solo HTTP/HTTPS**; esquemas `file://`, `gopher://`, etc. denegados.
- **Timeout** global p. ej. **30 s** conexión + lectura.
- **Tamaño:** truncar o abortar al superar `max_bytes` (tabla §1).
- **Redirecciones:** máx. **5** hops; tras cada hop, **re-validar** host contra reglas.
- **SSRF básico:** denegar destinos **RFC1918**, **localhost**, **metadata** (169.254.169.254), **enlace local IPv6**, y opcionalmente lista bloqueada de **DNS que resuelve a IP privada** (best-effort).
- **Sin** credenciales en URL; no enviar cookies de sesión del usuario.

Detalle evolutivo: [ARCHITECTURE.md](ARCHITECTURE.md) §8.2 (hueco “política de red”).

---

## 3. Tool calling (**D2**)

- **Preferido:** formato nativo del proveedor (p. ej. herramientas en API Ollama/OpenAI-compatible cuando existan y sean estables con vuestro modelo **D11**).
- **Plan B obligatorio:** mensaje del asistente que incluya bloque JSON o formato acordado con lista de `tool_calls`; el orquestador parsea y valida contra esta tabla; si falla, **un** reintento con recordatorio de esquema (ver [ARCHITECTURE.md](ARCHITECTURE.md) **D2**).

---

## 4. Presupuesto de bucle (relación §8.2)

Valores iniciales sugeridos (ajustar tras uso real):

| Recurso | Valor MVP | Comentario |
|---------|-----------|------------|
| Máx. **iteraciones LLM** por mensaje de usuario | **32** | Incluye turnos con `tool_results`; cortar con error explícito |
| Máx. **invocaciones de tools** por mensaje de usuario | **64** | Evita bucles solo-tools |
| Reintentos por **llamada** al LLM | Ver [RETRY_LOGIC.md](RETRY_LOGIC.md) **D22** | No confundir con reintentos de `web_fetch` |

---

## 5. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: tabla MVP read/bash/web_search/web_fetch; §2 red; §3 D2; §4 presupuesto bucle. |
