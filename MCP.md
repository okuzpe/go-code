# MCP (Model Context Protocol) — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.8](ARCHITECTURE.md). Especificación oficial: [Model Context Protocol](https://modelcontextprotocol.io/specification/2025-11-25); implementación de referencia analizada en terceros: [MCP — claude-code-explain](https://claude-code-explain.helmcode.com/mcp).

En **2025–2026**, un asistente “agente” que solo expone herramientas integradas **sin** MCP suele quedar por debajo de la expectativa de usuarios y de integradores (GitHub, Slack, navegador, bases de datos, etc.). MCP es el **adaptador estándar** para conectar el modelo a procesos y servicios externos con un contrato común.

---

## 1. Qué resuelve MCP

- **Servidores** (subprocess **stdio**, o remotos **SSE** / **HTTP** streamable / **WebSocket**) publican **herramientas**, **recursos** y a veces **prompts**.
- El **cliente** (nuestro CLI) mantiene sesiones, traduce llamadas del modelo a `callTool`, aplica **permisos** igual que al resto de tools, y gestiona **auth**, timeouts y límites de salida.

No sustituye las herramientas **builtin** (`Read`, `Grep`, …): las **complementa** para lo que no queréis mantener en el binario.

---

## 2. Convención de nombres expuesta al modelo

Cada tool MCP se expone al LLM con un nombre normalizado (referencia Claude Code):

```text
mcp__<server>__<tool>
```

- Caracteres fuera de `[a-zA-Z0-9_-]` → `_`, longitud acotada (p. ej. **64** caracteres en referencia).
- Las reglas `allow` / `deny` de permisos usan estos nombres (incl. wildcards tipo `mcp__slack__*`).

**Eco Go:** función pura `NormalizeMCPToolName(server, tool string) string` compartida entre `internal/mcp` y `internal/permissions`.

---

## 3. Alcance de configuración (prioridad, referencia)

Orden típico de fusión (de menor a mayor prioridad; el último gana en conflicto de **nombre** de servidor):

| # | Scope | Origen (referencia) | Notas |
|---|--------|----------------------|--------|
| 1 | org / cloud | Conectores desde API | Producto cerrado |
| 2 | plugin | Plugins instalados | Ver [PLUGINS.md](PLUGINS.md) |
| 3 | user | `~/.claude/settings.json` | Global usuario |
| 4 | project | `.mcp.json` (subiendo hasta home) | Requiere **aprobación explícita** antes de conectar en referencia |
| 5 | local | `settings.local.json` | No committed |
| 6 | enterprise | `managed-mcp.json` | **Exclusivo:** si existe, ignora otros scopes |

**Eco Go:** equivalente en `~/.config/assistant/…` + `.assistant/.mcp.json` (nombres **D7**); flag **workspace trust** antes de cargar MCP de proyecto; política enterprise opcional muy tardía.

---

## 4. Transportes

| Transporte | Uso típico | Notas (referencia) |
|------------|------------|---------------------|
| **stdio** | `command` + `args`; subprocess | Más común; stderr acotado para debug; timeout de conexión orden 30 s |
| **sse** | URL remota, EventSource | Sin timeout en el stream; requests HTTP ~60 s |
| **http** | Streamable HTTP (spec 2025-03-26) | Sesión, OAuth |
| **ws** | WebSocket | TLS, proxy |

Expansión de variables en config: `${VAR}` y `${VAR:-default}`.

**Eco Go:** MVP interno razonable = **stdio + un remote mínimo** cuando **D6** lo apruebe; alinear timeouts con [Context](https://pkg.go.dev/context) y política de red (SSRF ya tratada para `web_fetch`).

---

## 5. Autenticación (referencia)

- **OAuth** (SSE/HTTP): flujo navegador, tokens en almacén seguro, refresh en 401, revocación RFC 7009.
- **XAA (Cross-App Access):** intercambio de tokens (RFC 8693, RFC 7523); un popup puede autenticar varios servidores; suele ir tras feature flag / entorno enterprise.
- **McpAuthTool (pseudo-tool):** si el servidor no conecta por auth, se expone `mcp__<server>__authenticate`; el modelo la invoca y se lanza el flujo; en referencia suele **auto-aprobarse**; fallos cacheados ~**15 min**.

---

## 6. Ciclo de vida (resumen)

**Arranque:** cargar scopes → deduplicar por firma (URL vs comando) → filtrar políticas enterprise → aprobar servidores de proyecto → conectar en paralelo (concurrencia mayor para remotos que para stdio en referencia) → obtener tools / resources / prompts.

**Llamada:** `tool_use` con nombre `mcp__…` → asegurar cliente conectado → **permissions** → `callTool` con timeout → si la salida supera umbral (~**100k** caracteres en referencia), escribir a temporal y devolver instrucción para leer con **Read**.

**Parada:** stdio: señales escalonadas SIGINT → SIGTERM → SIGKILL en ventanas cortas; remotos: cerrar transport y rechazar pendientes.

---

## 7. Permisos y políticas

- En referencia, MCP **passthrough** al sistema de permisos global: interactivo pregunta, bypass auto-aprueba, auto-modo puede pasar por clasificador (**D17**).
- Reglas usuario: `allow` / `deny` con prefijos `mcp__…`.
- Enterprise: listas `allowedMcpServers` / `deniedMcpServers` (nombre, URL pattern, comando); **deny gana**.

---

## 8. Roadmap propuesto (Go)

| Fase | Alcance MCP |
|------|-------------|
| **MVP** | Ninguno obligatorio: consolidar bucle + tools builtin. |
| **v2 (recomendado si producto “agente completo”)** | `internal/mcp`: cliente **stdio**, registro dinámico de tools `mcp__*`, límites de salida, integración **permissions**, config usuario/proyecto mínima; pseudo-tool de auth opcional o segunda iteración. |
| **v3+** | Transportes **SSE/HTTP/WS**, OAuth estable, resources/list/read, prompts como comandos, merge con **plugins** (**D20**), headers dinámicos — según **D6**. |

**D6** debe fijar: transportes en v2, compatibilidad con `.mcp.json`, y si exigís paridad con políticas enterprise desde el día uno.

---

## 9. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: naming, scopes, transportes, auth, ciclo de vida, permisos, roadmap v2/v3, **D6** |
