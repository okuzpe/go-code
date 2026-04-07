# OpenClaw — Agentes, herramientas, web y tubería de respuesta (profundidad)

**Nota:** en este workspace **no** hay carpeta `openclaw/`; el código de referencia está en [openclaw/openclaw](https://github.com/openclaw/openclaw) (`src/agents/`, `src/auto-reply/`, …).

Complemento de [ARCHITECTURE.md](ARCHITECTURE.md) y [OPENCLAW_REFERENCE.md](OPENCLAW_REFERENCE.md). Se centra en lo más cercano a un **asistente agente en terminal** que queremos implementar en **Go**: herramientas, búsqueda/fetch, sandbox, auto-respuesta y contexto.

**Principio reutilizado de productos tipo Claude Code:** preferir **herramientas dedicadas** (`Read`, `Grep`, etc.) sobre `Bash` equivalente; la taxonomía y roadmap en Go están en [ARCHITECTURE.md §2.1](ARCHITECTURE.md).

**Perfiles de agente** (modelo + subset de herramientas + modo de permisos + qué contexto cargar): [ARCHITECTURE.md §2.7](ARCHITECTURE.md) y [AGENT_PROFILES.md](AGENT_PROFILES.md).

**Multi-agente** (Coordinator hub-and-spoke **vs** Team/Swarm peer-to-peer; no unificar en un solo “modo”): [ARCHITECTURE.md §2.11](ARCHITECTURE.md) y [COORDINATOR_MODE.md](COORDINATOR_MODE.md).

**Agentes declarativos** (`*.md` + frontmatter, prioridad sobre built-ins): [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) (**D19**).

**Editor local (VS Code / Cursor, MCP localhost):** patrón CLI ↔ IDE en [IDE_BRIDGE.md](IDE_BRIDGE.md) (**D21**); OpenClaw puede exponer gateway/MCP en otras capas — no confundir con el Bridge remoto a UI web.

**Cliente MCP (integraciones externas):** [MCP.md](MCP.md) (**D6**); el árbol OpenClaw incluye `mcp/` y `agents/mcp-*.ts` como referencia de implementación.

**Comportamiento observable (explorar → implementar):** [PRACTICAL_TIPS.md](PRACTICAL_TIPS.md) resume decisiones de UX/coste/seguridad del producto analizado.

**Fiabilidad de llamadas al modelo:** [RETRY_LOGIC.md](RETRY_LOGIC.md) (**D22**); el runner/auto-reply debe tolerar 429/5xx sin perder el hilo de sesión.

**Índice explainer ↔ nuestros .md:** [DOCS_MAP.md](DOCS_MAP.md).

---

## 1. Paquete `src/agents/`

Subdirectorios relevantes (ver árbol local):

| Subcarpeta | Propósito típico |
|------------|------------------|
| `tools/` | Implementación de herramientas del modelo: web, fetch, generación avanzada, etc. |
| `skills/` | Integración de skills con el agente |
| `schema/` | Esquemas / contratos de herramientas y validación |
| `sandbox/` | Aislamiento de ejecución (cuando aplica) |
| `cli-runner/`, `pi-embedded-runner/`, `command/` | Ejecución del agente desde CLI o entornos embebidos |
| `auth-profiles/` | Perfiles de autenticación hacia proveedores |

**Eco Go:** paquetes `internal/tools`, `internal/orchestrator`, `internal/llm`, más `internal/permissions` alrededor de cada ejecución de herramienta.

---

## 2. Herramientas web (`agents/tools/` + `web-search/` + `web-fetch/`)

OpenClaw separa y prueba con rigor:

- **Búsqueda** (`web-search.ts`, providers, citas/redirecciones).
- **Fetch** (`web-fetch.ts`, `web-guarded-fetch.ts`) con énfasis en **SSRF**, visibilidad y límites.

Archivos de test representativos (idea de madurez): `web-fetch.ssrf.test.ts`, `web-fetch.provider-fallback.test.ts`.

**Para nuestro diseño**

- Copiar **criterios**, no código: allowlist/denylist de hosts, límites de tamaño, timeouts, y tests de SSRF desde el primer día si exponemos `web_fetch` a URLs arbitrarias.
- Alineado con decisión **D3** en [ARCHITECTURE.md](ARCHITECTURE.md) (proveedor de búsqueda vs. fuente acotada).

---

## 3. Auto-respuesta (`src/auto-reply/`)

Incluye `reply/queue`, `reply/exec`, comandos ACP y subagentes. Es la tubería que conecta **mensaje entrante** → **trabajo del agente** → **respuesta**.

**Eco Go:** un único bucle:

1. Entrada de usuario (REPL).
2. Llamada al proveedor LLM con definición de herramientas.
3. Ejecución de herramientas bajo política de permisos.
4. Reenvío de `tool_results` hasta respuesta final.

Los detalles de cola y comandos ACP de OpenClaw son **referencia** para cuando necesitemos comandos slash, hooks o sub-agentes.

---

## 4. Motor de contexto (`src/context-engine/`)

Responsable del ensamblaje y poda del contexto que ve el modelo (nombre explícito “engine”).

**Eco Go:** módulo `session`: historial, micro-compactado de salidas de herramientas, posible resumen bajo umbral de tokens — [ARCHITECTURE.md §2.5](ARCHITECTURE.md) y detalle en [CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md).

---

## 5. Cron y agente aislado (`src/cron/`)

Incluye `isolated-agent` para tareas fuera del camino interactivo.

**Eco Go:** fase posterior (scheduler + mismo orquestador con `context` cancelable).

---

## 6. MCP (`src/mcp/`)

Integración directa en el árbol fuente además del enfoque **mcporter** citado en VISION.

**Eco Go:** cuando D6 sea “sí”, un cliente MCP (p.ej. SDK oficial Go) detrás del mismo `ToolRegistry`, con permisos unificados.

---

## 7. Seguridad (`src/security/`)

Políticas transversales; en VISION se insiste en **seguridad y valores por defecto** primero.

**Eco Go:** validación de shell, confirmación explícita, documentación de riesgo en Windows vs. Unix (sandbox limitado), y en **auto-modo** un segundo riel de decisión tipo [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) (consulta lateral, no solo heurísticas locales).

---

## 8. Hooks (`src/hooks/`)

Hooks de ciclo de vida (incluye `hooks/bundled` con documentación README).

**Eco Go:** eventos locales (pre/post tool) como extensión; no bloquear MVP. Patrón y eventos del producto analizado: [HOOKS.md](HOOKS.md); decisión de producto **D18** en [ARCHITECTURE.md §5](ARCHITECTURE.md).

---

## 9. Checklist de inspiración → decisiones en Go

Huecos prioritarios y estrategia de documentación: ver **§8** en [ARCHITECTURE.md](ARCHITECTURE.md).

- [x] Lista cerrada **MVP:** [TOOL_CONTRACT.md](TOOL_CONTRACT.md) §1 (`read_file`, `bash`, `web_search`, `web_fetch`); v1 añadirá `glob`/`grep`.
- [x] Política red **MVP:** [TOOL_CONTRACT.md](TOOL_CONTRACT.md) §2 (SSRF básico, tamaño, redirecciones).
- [ ] Mapear “proveedor de búsqueda” a interfaz Go + 1 implementación mínima.
- [ ] Permissions: tabla Allow/Ask/Deny por herramienta y modo (ver ARCHITECTURE §5); si hay auto-modo, flujo con fast paths + clasificador ([YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md), **D17**).
- [ ] Tests de seguridad mínimos para fetch y shell.

---

## 10. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación del documento. |
| 2026-04-07 | Checklist enlaza §8 de ARCHITECTURE (auditoría / imperdibles). |
| 2026-04-07 | Intro: enlace a **§2.1** regla de herramientas dedicadas. |
| 2026-04-07 | Intro: enlace a **§2.7** y [AGENT_PROFILES.md](AGENT_PROFILES.md). |
| 2026-04-07 | §7 seguridad + checklist: [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) / D17. |
| 2026-04-07 | §8 hooks: enlaces [HOOKS.md](HOOKS.md) y D18. |
| 2026-04-07 | Intro: [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) / D19. |
| 2026-04-07 | Intro: [PRACTICAL_TIPS.md](PRACTICAL_TIPS.md). |
| 2026-04-07 | Intro: [RETRY_LOGIC.md](RETRY_LOGIC.md) / D22. |
| 2026-04-07 | Intro: [DOCS_MAP.md](DOCS_MAP.md). |
| 2026-04-07 | Checklist §9: ítems contrato + red marcados vía [TOOL_CONTRACT.md](TOOL_CONTRACT.md). |
| 2026-04-07 | Cabecera: sin `openclaw/` local; puntero GitHub. |
