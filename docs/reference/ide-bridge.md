# Integración IDE (local) y Bridge (remoto) — referencia y eco Go

**Status in goclaw:** **Partial** — lockfile MCP discovery, `GOCLAW_IDE_NOTIFY_URL`; see **D21** / §6–§7 in this file and [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md).

Profundidad ligada a [CLAUDE.md](../../goclaw/CLAUDE.md) (IDE / D21). Referencia (terceros, análisis de Claude Code): [Bridge & IDE — claude-code-explain](https://claude-code-explain.helmcode.com/bridge-ide).

Son **dos sistemas distintos**. Mezclarlos en diseño lleva a confusiones de seguridad (localhost de confianza vs túnel a Internet y OAuth).

**Relación con [mcp.md](./mcp.md):** el IDE integra al CLI como **cliente MCP hacia el editor** (localhost). Los **servidores MCP de integración** (GitHub, Slack, …) son otro eje: el CLI como **cliente** hacia procesos/URLs configurados en `mcpServers` — ver **D6** en [CLAUDE.md](../../goclaw/CLAUDE.md) y transportes en [mcp.md](./mcp.md).

---

## 1. Dos sistemas (no uno)

| Sistema | Actores | Transporte | Objetivo |
|---------|---------|------------|----------|
| **Integración IDE** | Editor (VS Code, JetBrains, …) y **CLI en la misma máquina** | **MCP sobre localhost** (SSE o WebSocket) | Sincronizar ediciones, diffs, contexto (selección, archivos abiertos, cursor) |
| **Bridge** | Navegador (p. ej. claude.ai), backend del proveedor, **CLI como worker** | HTTPS / WSS / SSE según versión de protocolo | Control remoto del CLI desde la web, sesiones compartidas, permisos `can_use_tool` en el navegador |

**Prioridad para nuestro asistente en Go**

- **Integración IDE:** tratada como **paso fuerte** tras el núcleo REPL (**D21**): muchos usuarios viven en VS Code / Cursor / Windsurf; sin este puente el CLI queda aislado del flujo de edición.
- **Bridge análogo al de Anthropic:** **no** es paridad 1:1 con el producto de referencia; implica identidad OAuth, infraestructura ajena y semántica de sesión web. Documentar como **referencia**; un “bridge propio” (UI web + worker) sería **fase posterior** y diseño aparte.

---

## 2. Integración IDE (local)

### 2.1 Idea de arquitectura (referencia)

- Las **extensiones de IDE** levantan un **servidor MCP** en `localhost` con transporte interno tipo **`sse-ide`** (EventSource / SSE) o **`ws-ide`** (WebSocket); a veces con **token de autenticación** en cabecera o query.
- El **CLI descubre** instancias del editor leyendo **lockfiles** en un directorio (en referencia: `~/.claude/ide/<ide-name>-<pid>.lock`) que el IDE crea al arrancar la extensión y borra al cerrar. Cada lockfile lleva **puerto**, **tipo de transporte** y metadatos.
- El CLI actúa como **cliente MCP** hacia ese endpoint local para:
  - **Notificar ediciones** (el usuario acepta/rechaza en un diff del editor).
  - **Compartir contexto** hacia el agente (selección, pestañas abiertas, posición del cursor).
  - **Abrir/cerrar pestañas de diff** (`openDiffInIde`, `closeDiffTabsInIde` en referencia).
- Puede existir un **canal bidireccional** adicional (p. ej. notificaciones propias entre CLI y extensión) para feature flags, telemetría opcional o acciones rápidas — conviene **acotar** qué es MCP estándar vs extensión privada (**D21**).

### 2.2 Editores (referencia)

- **VS Code / Cursor / Windsurf:** extensión que puede instalarse automáticamente (`code --install-extension …` en el flujo de referencia).
- **JetBrains (varias IDEs):** plugin **manual**; misma idea de localhost + descubrimiento vía lockfiles si el ecosistema lo estandariza.

### 2.3 Superficie de amenaza (local)

- **Localhost no es “gratis”:** otros procesos en la misma máquina pueden intentar conectar al puerto del IDE; por eso **token en `ws-ide`** y binds acotados importan.
- Los **diffs y aceptación de cambios** deben enlazarse con la capa de **permisos** del CLI ([CLAUDE.md](../../goclaw/CLAUDE.md)): una edición “desde el agente” puede seguir pasando por **Ask** / políticas según modo.

---

## 3. Bridge (remoto) — referencia

### 3.1 Qué aporta frente al CLI solo

- El CLI pasa a ser **controlado desde el navegador**; el worker puede vivir en **otra máquina** (servidor, CI interactiva, etc.).
- **Permisos remotos:** el flujo `can_use_tool` envía una petición de control al backend; la UI muestra Allow/Deny y puede **devolver argumentos mutados** o **reglas nuevas** de sesión — patrón útil para copiar en una UI propia, **no** dependiente de Anthropic.
- **Modos de arranque** (referencia): proceso dedicado multi-sesión, bridge dentro del REPL, u modo “sin env” con conexión directa a sesión; **worktree** para aislar sesiones paralelas en Git.

### 3.2 Versiones de protocolo (resumen)

| Versión | Lectura (entrante al CLI) | Escritura (saliente) | Notas |
|---------|---------------------------|----------------------|--------|
| **v1** | WebSocket | HTTP POST con batching (~100 ms) | Polling/trabajo vía “Environments API” en referencia; más complejo |
| **v2** | SSE | HTTP POST (cliente unificado) | Creación de sesión más directa; a veces tras **feature gate** en referencia |

Constantes típicas (orden de magnitud en referencia): backoff de reconexión SSE 1 s → 30 s, *give up* ~10 min; *liveness* ~45 s sin datos; refresh JWT ~5 min antes de expirar; hasta **3 fallos** de init consecutivos deshabilitan el bridge.

### 3.3 Autenticación (referencia)

Orden de resolución orientativa: variable de entorno (solo dev) → cadena de credenciales del SO → OAuth interactivo. Cabeceras comunes hacia API: `Authorization: Bearer`, versionado API, *betas* corporativos, y en algunos niveles token de dispositivo de confianza.

**Eco Go:** si algún día se implementa “bridge propio”, **no** acoplar el núcleo del orquestador a OAuth de un tercero; aislar en `internal/remoteui` o similar.

---

## 4. Eco Go (resumen)

| Pieza | Ubicación sugerida | Notas |
|-------|-------------------|--------|
| Descubrimiento de IDE | `internal/ide/discovery.go` | Escanear directorio de lockfiles (ruta bajo **D21**, p. ej. `~/.config/assistant/ide/`); invalidar al borrar PID |
| Cliente MCP → IDE | `internal/ide/mcpclient.go` (o subpaquete) | SSE/WS + auth; timeouts; **solo localhost** por defecto |
| Contrato con orquestador | Interfaz inyectada desde `main` | p. ej. `IDENotifier`, `IDEContextProvider`; **evitar** `ide` → `orchestrator` |
| Herramientas Write/Edit | Tras éxito lógico | Llamar notificación para diff en editor cuando **D21** y sesión IDE activa |
| Bridge remoto | Fuera del MVP / v1 IDE | Si se hace: paquete separado; mismo orquestador; distinto “channel” de entrada |

**Roadmap sugerido:** **MVP** — solo REPL y permisos; **v1** — `glob`/`grep`/memoria etc. según [CLAUDE.md](../../goclaw/CLAUDE.md); **v1+ (prioridad producto)** — `internal/ide` con descubrimiento + cliente MCP mínimo + una extensión o adaptador documentado (**D21**). Bridge estilo claude.ai **sin compromiso**.

---

## 6. goclaw implementation (minimal, English)

**Current code:** [`goclaw/internal/ide/notify.go`](../../goclaw/internal/ide/notify.go), [`goclaw/internal/ide/discovery.go`](../../goclaw/internal/ide/discovery.go).

- Environment variable **`GOCLAW_IDE_NOTIFY_URL`**: if set to `http` or `https` and the host is **`127.0.0.1`**, **`localhost`**, or **`::1`**, the orchestrator’s **after-tool** callback issues a **best-effort POST** with JSON `{"tool", "result_bytes", "is_error"}` after each tool completes. Remote URLs are rejected (no-op notifier).
- **`ide_bridge_mcp`:** when `true` in merged `settings.json`, goclaw scans **`~/.goclaw/ide/*.json`** (sorted by name), reads the first file with a valid **loopback** `url` and optional `headers`, and appends a synthetic MCP server **`id: "ide"`** before connecting (same Streamable HTTP stack as `mcp_servers[].url` in [`goclaw/internal/mcp/http.go`](../../goclaw/internal/mcp/http.go)). Extensions can drop a lockfile such as `{"url":"http://127.0.0.1:1234/mcp","headers":{"Authorization":"Bearer …"}}`.
- **D21 “full bridge”** still depends on editor-side MCP servers and UX; goclaw provides HTTP client + discovery, not IDE UI.

---

## 7. goclaw ↔ extension contract (V3 spec)

**Goal:** keep editor extensions and goclaw aligned without coupling the orchestrator to a specific IDE.

| Concern | Contract |
|---------|-----------|
| **MCP endpoint** | Extension writes a JSON lockfile under `~/.goclaw/ide/*.json` with `url` (loopback `http`/`https`) and optional `headers`. User sets `ide_bridge_mcp: true`; goclaw appends synthetic MCP server `id: "ide"` and uses the same streamable HTTP client as other `mcp_servers[].url` entries. |
| **Bearer / auth** | Prefer `headers.Authorization` in the lockfile, or use `mcp_servers` with `bearer_token_file` for static tokens (see [`docs/goclaw/mcp-remote.md`](../goclaw/mcp-remote.md)). |
| **Post-tool notify** | `GOCLAW_IDE_NOTIFY_URL` must stay **loopback-only**. Payload shape is stable JSON: `{"tool","result_bytes","is_error"}`. Extensions may use this for progress UI without MCP. |
| **Future events** | Additional notification types should be versioned (e.g. `event` field) and remain **best-effort**; the REPL must not depend on the IDE replying. |
| **OAuth / remote IDE** | Out of scope until explicitly designed; same posture as D6 OAuth for MCP. |

**Reference implementation:** [`goclaw/internal/ide/discovery.go`](../../goclaw/internal/ide/discovery.go), [`goclaw/internal/ide/notify.go`](../../goclaw/internal/ide/notify.go), wiring in [`goclaw/internal/app/chat_wiring.go`](../../goclaw/internal/app/chat_wiring.go).

---

## 5. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: IDE local vs Bridge remoto, descubrimiento, transports, amenaza localhost, eco Go, **D21** |
| 2026-04-07 | §6: `GOCLAW_IDE_NOTIFY_URL`, `internal/ide/notify.go`, scope vs full D21. |
| 2026-04-09 | §6: `ide_bridge_mcp`, `internal/ide/discovery.go`, lockfile JSON → MCP HTTP. |
| 2026-04-09 | §7: goclaw ↔ extension contract (lockfile MCP, notify payload, future versioning). |
