# OpenClaw — mapa del repositorio y extracción para nuestro diseño (Go)

**Estado en este workspace:** **no** hay carpeta local `openclaw/`. Este archivo y [OPENCLAW_GATEWAY_CHANNELS.md](OPENCLAW_GATEWAY_CHANNELS.md), [OPENCLAW_AGENTS_AND_TOOLS.md](OPENCLAW_AGENTS_AND_TOOLS.md) son **notas de arquitectura** basadas en el producto upstream; los enlaces de rutas apuntan a **[openclaw/openclaw en GitHub](https://github.com/openclaw/openclaw)** (`main`). Sitio de documentación: [docs.openclaw.ai](https://docs.openclaw.ai). OpenClaw **no** es el código que vamos a portar a Go: es **referencia de modularización y riesgos**.

**Aclaración:** OpenClaw **no** es el **Claude Code CLI** (Anthropic). Es otro producto (Gateway, multicanal, Node/TS) con patrones útiles (agente ↔ herramientas, permisos, MCP). El **código relacionado en local** que sí tenéis en `go-code/` es [claw-code/](claw-code/) (otro subárbol — parity/TUI/Rust); no sustituye a este mapa.

**Profundidad adicional**

- [OPENCLAW_GATEWAY_CHANNELS.md](OPENCLAW_GATEWAY_CHANNELS.md) — gateway, daemon, canales y enrutado.
- [OPENCLAW_AGENTS_AND_TOOLS.md](OPENCLAW_AGENTS_AND_TOOLS.md) — agentes, herramientas, web, sandbox, auto-respuesta, MCP.

---

## 1. Qué es OpenClaw (contexto)

- Asistente personal **auto-hospedado**: un proceso “Gateway” conecta **muchas apps de mensajería** a un **agente con herramientas** (documentación: [docs/index.md en GitHub](https://github.com/openclaw/openclaw/blob/main/docs/index.md)).
- Runtime declarado en README: **Node 24 (recomendado) u 22.16+**; empaquetado como paquete npm (`openclaw`, entrada `openclaw.mjs`).
- Prioridades explícitas en [VISION.md](https://github.com/openclaw/openclaw/blob/main/VISION.md): seguridad y valores por defecto, estabilidad, UX de primer arranque; memoria y plugins como extensiones; MCP integrado vía **mcporter** (desacoplado del núcleo).

**Implicación para nuestro proyecto en Go:** nos interesa sobre todo el subconjunto **“agente + herramientas + permisos + sesión”**, no necesariamente la paridad multi-canal ni el ecosistema npm de extensiones.

---

## 2. Layout del monorepo

| Ruta (en repo upstream) | Rol |
|------|-----|
| [`package.json`](https://github.com/openclaw/openclaw/blob/main/package.json) | Raíz del paquete publicado, `bin.openclaw`, exports desde `dist/`, inclusión de `docs/` y `skills/`. |
| [`pnpm-workspace.yaml`](https://github.com/openclaw/openclaw/blob/main/pnpm-workspace.yaml) | Workspace: `.`, `ui`, `packages/*`, `extensions/*`. |
| [`src/`](https://github.com/openclaw/openclaw/tree/main/src) | Código principal del gateway, CLI, agentes, canales, etc. (~90 entradas de primer nivel; ver §3). |
| [`packages/`](https://github.com/openclaw/openclaw/tree/main/packages) | SDKs / contratos publicables (`plugin-sdk`, `clawdbot`, `memory-host-sdk`, `moltbot`, `plugin-package-contract`). |
| [`extensions/`](https://github.com/openclaw/openclaw/tree/main/extensions) | Extensiones npm (orden ~**10²** carpetas: proveedores LLM, canales, búsqueda, memoria, voz…). |
| [`apps/`](https://github.com/openclaw/openclaw/tree/main/apps) | Apps nativas (macOS, iOS, Android) — fuera del núcleo CLI mínimo. |
| [`ui/`](https://github.com/openclaw/openclaw/tree/main/ui) | Frontend del control UI web. |
| [`skills/`](https://github.com/openclaw/openclaw/tree/main/skills) | Skills empaquetadas / base (VISIÓN: skills en hub más que en core). |
| [`docs/`](https://github.com/openclaw/openclaw/tree/main/docs) | Documentación del producto (volumen alto). |
| [`test/`](https://github.com/openclaw/openclaw/tree/main/test), [`test-fixtures/`](https://github.com/openclaw/openclaw/tree/main/test-fixtures), [`qa/`](https://github.com/openclaw/openclaw/tree/main/qa) | Pruebas, fixtures y QA. |
| [`scripts/`](https://github.com/openclaw/openclaw/tree/main/scripts) | Scripts de build, tooling, dock helpers. |
| [`assets/`](https://github.com/openclaw/openclaw/tree/main/assets) | Recursos estáticos. |
| [`vendor/`](https://github.com/openclaw/openclaw/tree/main/vendor) | Dependencias vendoreadas si aplica. |
| [`patches/`](https://github.com/openclaw/openclaw/tree/main/patches) | Parches pnpm/npm. |
| [`git-hooks/`](https://github.com/openclaw/openclaw/tree/main/git-hooks) | Hooks de git del repo. |
| [`Swabble/`](https://github.com/openclaw/openclaw/tree/main/Swabble) | Subproyecto / herramienta relacionada (no núcleo agente mínimo). |

**Nota:** También hay `src/plugin-sdk/` (código en el árbol principal) **además de** `packages/plugin-sdk/` (paquete del workspace): el primero suele ser runtime/contratos acoplados al gateway; el segundo es el SDK que consumen extensiones.

---

## 3. `src/` — directorios que más nos importan

Hay **decenas** de carpetas bajo `src/`; aquí el subconjunto que **mapea mejor** al MVP Go de [ARCHITECTURE.md](ARCHITECTURE.md) §4.1. El resto (media, voz, canvas, infra…) es producto OpenClaw amplio; no hace falta portarlo para un CLI tipo “Claude Code lite”.

| Ruta en `src/` | Contenido típico | Eco Go (conceptual) |
|----------------|------------------|---------------------|
| `agents/` | Orquestación del agente, `tools/`, `skills/`, `schema/`, `sandbox/`, runners embebidos | `orchestrator`, `tools`, `skills`, `sandbox` (opcional) |
| `agents/tools/` | Web search/fetch con tests SSRF, providers, guarded fetch | `tools/websearch`, `tools/webfetch`, políticas de red |
| `auto-reply/` | Cola, ejecución, comandos ACP/subagentes | Bucle “mensaje → modelo → herramientas → respuesta” |
| `gateway/` | Protocolo, servidor, métodos RPC del gateway | Solo si hiciéramos servidor de control; CLI mínimo puede omitir |
| `channels/` | Plugins de transporte, allowlists, acciones | Opcional al inicio: solo REPL/stdin |
| `cli/` | Programa CLI, daemon, gateway-cli | `cmd/`, `internal/cli` |
| `commands/` | Comandos slash / registrados (ecosistema OpenClaw) | v3+ plugins / CLI propia; no MVP |
| `config/` | Configuración y sesiones | `internal/config` |
| `context-engine/` | Motor de contexto (podado, ensamblaje) | `internal/session`, compactación — [CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md) |
| `cron/` | Tareas programadas | Fase posterior |
| `daemon/` | Servicio en segundo plano | Opcional |
| `hooks/` | Hooks de sesión y eventos (incl. bundled bajo `hooks/bundled/`) | `internal/hooks` — [HOOKS.md](HOOKS.md) |
| `mcp/` | Cliente / integración MCP en core | `internal/mcp` (**no** usar la ruta inexistente `src/services/mcp/`) |
| `plugins/` | Runtime de carga de plugins en proceso | `internal/plugin` (modelo distinto en Go) — [PLUGINS.md](PLUGINS.md) |
| `plugin-sdk/` | Parte core del contrato plugin (vs `packages/plugin-sdk`) | API estable extensiones; en Go a menudo MCP |
| `routing/` | Enrutado de mensajes / agentes | Simplificado si hay un solo canal |
| `security/` | Políticas de seguridad | `internal/permissions` + validación shell |
| `sessions/` | Sesiones de conversación | `internal/session` |
| `web-search/`, `web-fetch/` | Lógica web compartida (además de `agents/tools`) | Ideas para D3 / límites |
| `wizard/` | Onboarding interactivo | CLI `init` / flags |
| `tui/` | TUI rica | REPL simple o biblioteca TUI |
| `pairing/`, `node-host/` | Nodos móviles / emparejamiento | Fuera de scope inicial |
| `acp/` | ACP control plane / runtime | Avanzado |

**Otros directorios en `src/` (referencia rápida):** `infra/`, `process/`, `terminal/`, `tasks/`, `secrets/`, `web/`, `utils/`, `shared/`, `types/`, `logging/`, `markdown/`, `chat/`, `flows/`, `interactive/`, `bindings/`, `bootstrap/`, `compat/`, `canvas-host/`, `memory-host-sdk/`, y familias **media** (`media/`, `media-generation/`, `media-understanding/`, `image-generation/`, `video-generation/`, `music-generation/`, `realtime-transcription/`, `realtime-voice/`, `tts/`, …). Sirven al producto completo OpenClaw (voz, canvas, multi-dispositivo), no al **primer binario** descrito en §4.4 de ARCHITECTURE.

**Conclusión para vuestro diseño:** **no** hace falta clonar OpenClaw en este workspace. Si re-clonáis upstream, **recomprobáis** rutas en GitHub. **Mantened** estos tres `.md` alineados si cambia el árbol típico de `src/` en releases mayores; **no asumáis** que todo `src/` es imprescindible para un asistente tipo CC en Go.

---

## 4. Extensiones (`extensions/*`)

Cada carpeta suele ser un paquete npm autocontenido (proveedor, canal o capacidad). Patrones útiles para nuestro diseño:

- **Proveedores LLM** (`anthropic`, `openai`, `ollama`, etc.) → en Go: interfaces `LLMProvider` + implementaciones.
- **Búsqueda** (`brave`, `duckduckgo`, `searxng`, `tavily`, `exa`) → decisión D3 en [ARCHITECTURE.md](ARCHITECTURE.md).
- **Memoria** (`memory-core`, `memory-lancedb`, …) → plugin único activo en OpenClaw; nosotros podemos posponer o usar un solo backend.

---

## 5. Paquetes internos (`packages/*`)

- `plugin-sdk` — contrato para extensiones (equivalente conceptual: API estable para “capacidades” externas, en Go a menudo **MCP** en lugar de npm).

---

## 6. Ideas transferibles vs. no transferibles (sin portar código)

**Transferibles (patrones / límites)**

- Separar **fetch** con políticas anti-SSRF y visibilidad (hay tests dedicados en `agents/tools`).
- Skills como artefactos declarativos fuera del binario principal.
- MCP como capa **desacoplada** del núcleo (OpenClaw menciona mcporter para no reiniciar gateway al cambiar servidores).
- Priorizar **seguridad por defecto** con knobs explícitos para flujos de confianza (VISION + SECURITY).

**No transferibles tal cual**

- Monorepo npm, carga dinámica de extensiones Node, Control UI completa, matriz de canales, apps móviles.

---

## 7. Enlaces útiles (GitHub `openclaw/openclaw`)

- [README.md](https://github.com/openclaw/openclaw/blob/main/README.md) — instalación, modelo, onboarding.
- [VISION.md](https://github.com/openclaw/openclaw/blob/main/VISION.md) — filosofía, plugins, skills, MCP.
- [AGENTS.md](https://github.com/openclaw/openclaw/blob/main/AGENTS.md) — guía para quien trabaja en el repo upstream.
- [`docs/gateway/`](https://github.com/openclaw/openclaw/tree/main/docs/gateway) — documentación del gateway.

---

## 8. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Primera versión del mapa y tabla `src/` → eco Go. |
| 2026-04-07 | Alineación con árbol real: OpenClaw ≠ Claude Code CLI; checkout filtrado; §2 (extensiones ~10², carpetas raíz extra, `src/plugin-sdk/` vs `packages/plugin-sdk/`); §3 (`hooks/`, `commands/`, `mcp/` sin `services/mcp/`, bloque “otros directorios”). Coherencia: [ARCHITECTURE.md](ARCHITECTURE.md) §4.1 fila `mcp` sin ruta inexistente. |
| 2026-04-07 | **Sin `openclaw/` local:** cabecera + tablas §2/§7 enlazan a GitHub `main`; contraste con [claw-code/](claw-code/) en workspace. |
