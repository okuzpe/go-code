# Mapa de documentación ↔ [Claude Code Internals](https://claude-code-explain.helmcode.com/)

**Propósito:** una sola entrada para humanos e IA — qué tema del explainer cubre cada archivo nuestro y si aplica al **MVP** del binario Go. **Inventario por tiers** (Tier 1 en raíz incl. [TOOL_CONTRACT.md](TOOL_CONTRACT.md)): [ARCHITECTURE.md §8.0](ARCHITECTURE.md).

**Hub conceptual:** [ARCHITECTURE.md](ARCHITECTURE.md) (decisiones D0–D22, §3–§6).

---

## Orden de lectura sugerido (implementar MVP)

1. [ARCHITECTURE.md §1](ARCHITECTURE.md) — alcance
2. [ARCHITECTURE.md §3.1](ARCHITECTURE.md) — bucle orquestador
3. [ARCHITECTURE.md §4.4](ARCHITECTURE.md) — fila **MVP**
4. [ARCHITECTURE.md §5](ARCHITECTURE.md) — **D1–D5**, **D22** (mínimo)
5. [TOOL_CONTRACT.md](TOOL_CONTRACT.md) — contrato MVP + red + presupuesto bucle; §5.1 de [ARCHITECTURE.md](ARCHITECTURE.md)
6. Profundidad según implementación: [RETRY_LOGIC.md](RETRY_LOGIC.md), [LOCAL_MODELS.md](LOCAL_MODELS.md) si D1=local

---

## Tabla de cobertura (índice helmcode → repo)

| ID | Tema (helmcode) | Documento principal | Ancla / sección | MVP |
|----|-------------------|---------------------|-----------------|-----|
| 00 | Overview | [References.MD](References.MD) + [ARCHITECTURE.md](ARCHITECTURE.md) | §1, §2 | sí (contexto) |
| 01 | System Prompt | [ARCHITECTURE.md](ARCHITECTURE.md) | §2.6 | sí (capas mínimas en `internal/prompt`) |
| 02 | Tools | [ARCHITECTURE.md](ARCHITECTURE.md), [TOOL_CONTRACT.md](TOOL_CONTRACT.md) | §2.1; contrato MVP | sí |
| 03 | Agents | [AGENT_PROFILES.md](AGENT_PROFILES.md), [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) | — | MVP: un perfil; v1+ Explore/Plan |
| 04 | Memory | [MEMORY_SYSTEM.md](MEMORY_SYSTEM.md) | — | deferred (v1+ manual / índice) |
| 05 | Permissions | [ARCHITECTURE.md](ARCHITECTURE.md), [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) | §2.3, §2.4 | sí (D5) |
| 06 | Context & compaction | [CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md) | — | deferred (v1 compactación seria) |
| 07 | Costs | [PRACTICAL_TIPS.md](PRACTICAL_TIPS.md) §4; [COSTS.md](COSTS.md) stub | — | no (cloud); N/A local |
| 08 | Retry logic | [RETRY_LOGIC.md](RETRY_LOGIC.md) | — | sí (mínimo, **D22**) |
| 09 | Proactive / KAIROS | [ARCHITECTURE.md](ARCHITECTURE.md) | §2.18 | no |
| 10 | Hidden features | [ARCHITECTURE.md](ARCHITECTURE.md) | §2.18 | no |
| 11 | Practical tips | [PRACTICAL_TIPS.md](PRACTICAL_TIPS.md) | — | contexto |
| 12 | Skills | [SKILLS.md](SKILLS.md), [ARCHITECTURE.md](ARCHITECTURE.md) §2.9 | — | deferred (v3) |
| 13 | Slash commands | [ARCHITECTURE.md](ARCHITECTURE.md), [PLUGINS.md](PLUGINS.md) | §2.14, §2.18 | deferred (plugins/v3) |
| 14 | MCP | [MCP.md](MCP.md) | — | deferred (v2+, **D6**) |
| 15 | Bridge & IDE | [IDE_BRIDGE.md](IDE_BRIDGE.md) | — | v1+ (**D21**); no Bridge vendor |
| 16 | Coordinator Mode | [COORDINATOR_MODE.md](COORDINATOR_MODE.md) | — | deferred (v2+ **D16**) |
| 17 | YOLO Classifier | [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) | — | deferred (v1 fast paths; v2+ **D17**) |
| 18 | Hooks | [HOOKS.md](HOOKS.md) | — | deferred (v2+ **D18**) |
| 19 | Plugins | [PLUGINS.md](PLUGINS.md) | — | deferred (v3+ **D20**) |
| 20 | Custom agents | [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) | — | deferred (v3+ **D19**) |
| 21 | Bash security | [BASH_SECURITY.md](BASH_SECURITY.md), [ARCHITECTURE.md](ARCHITECTURE.md) §2.4 | — | sí (política + D4) |

**Leyenda MVP:** `sí` = entrar en el primer binario según §4.4; `deferred` = fase posterior explícita; `no` = fuera de alcance inicial; `contexto` = lectura útil, no bloque de código obligatorio.

---

## Enlaces directos al explainer (terceros)

Lista completa en [References.MD](References.MD).

---

## OpenClaw (notas upstream; código en GitHub)

Sin carpeta `openclaw/` en este workspace: [OPENCLAW_REFERENCE.md](OPENCLAW_REFERENCE.md), [OPENCLAW_GATEWAY_CHANNELS.md](OPENCLAW_GATEWAY_CHANNELS.md), [OPENCLAW_AGENTS_AND_TOOLS.md](OPENCLAW_AGENTS_AND_TOOLS.md) + [openclaw/openclaw](https://github.com/openclaw/openclaw). Código local aparte: [claw-code/](claw-code/) (no es OpenClaw).

---

## Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: tabla helmcode 00–21, orden MVP, enlaces; **SKILLS**/ **COSTS**/ **BASH_SECURITY** stubs enlazados |
| 2026-04-07 | Cross-links: [PRACTICAL_TIPS.md](PRACTICAL_TIPS.md) §4, [OPENCLAW_AGENTS_AND_TOOLS.md](OPENCLAW_AGENTS_AND_TOOLS.md); §2.6 ARCHITECTURE → explainer system-prompt |
| 2026-04-07 | Cabecera: puntero a inventario global §8.0 en [ARCHITECTURE.md](ARCHITECTURE.md) |
| 2026-04-07 | Orden MVP paso 5 → [TOOL_CONTRACT.md](TOOL_CONTRACT.md); fila **02** Tools enlaza contrato |
| 2026-04-07 | Sección OpenClaw: sin clon local; GitHub + [claw-code/](claw-code/) |
