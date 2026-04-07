# Tips prácticos (decisiones de producto visibles) — referencia y eco Go

Resumen de **diez comportamientos** que en Claude Code están **acoplados al código** (no son “trucos” de documentación). Fuente analizada: [Practical Tips — claude-code-explain](https://claude-code-explain.helmcode.com/tips). Cada tip enlaza con nuestros documentos de profundidad donde ya cubrimos el mismo tema.

**Leyenda:** estándar · **Atención** · **Peligro** (según impacto en coste, seguridad o pérdida de datos).

---

## 1. Reglas de repo en la capa alta del prompt

**Tip:** En referencia, `CLAUDE.md` en la raíz es de las **primeras** piezas que entran en contexto y orienta toda la sesión.

**Eco nosotros:** equivalente conceptual **`AGENTS.md`** / `CLAUDE.md` en la raíz del proyecto (convención ya citada en [MEMORY_SYSTEM.md](MEMORY_SYSTEM.md) §1 y [ARCHITECTURE.md §2.6](ARCHITECTURE.md)). Conviene **una sola fuente de verdad** para reglas de equipo: no duplicar en memoria lo que pertenece al repo.

---

## 2. Memoria entre sesiones

**Tip:** Persistencia bajo rutas tipo `~/.claude/projects/<slug>/memory/`; hechos que el usuario pide recordar **vuelven** en sesiones futuras.

**Eco nosotros:** [MEMORY_SYSTEM.md](MEMORY_SYSTEM.md), **D14**, `internal/memory`. Ajustar rutas a `~/.config/assistant/…` o `.assistant/memory` al implementar.

---

## 3. Agente Explore con modelo barato

**Tip:** **Explore** usa **Haiku** (rápido y barato) para búsquedas en código; delegar ahí ahorra tokens frente a usar el modelo principal para lo mismo.

**Eco nosotros:** [AGENT_PROFILES.md](AGENT_PROFILES.md) + §2.7 en [ARCHITECTURE.md](ARCHITECTURE.md); con **Ollama**, asignar un modelo **7B** al perfil Explore y reservar el grande para el bucle principal ([LOCAL_MODELS.md](LOCAL_MODELS.md)).

---

## 4. “Fast mode” ≠ modelo distinto (**Atención**)

**Tip:** `/fast` en referencia **no cambia** el modelo (p. ej. sigue siendo Opus): sube **prioridad de cómputo** y el **precio por token de entrada** (orden **6×** en la explicación analizada).

**Eco nosotros:** si ofrecéis modo “prioridad” sobre una API de pago, documentar **explícitamente** precio vs modelo; en local (Ollama) el análogo suele ser “más rápido” solo por cola/GPU, no por surcharges — no copiar la semántica de `/fast` sin leer la doc del proveedor.

Réplica de costes (referencia): [Costs — claude-code-explain](https://claude-code-explain.helmcode.com/costs); síntesis local: [COSTS.md](COSTS.md).

---

## 5. Auto-compact ~13K tokens libres

**Tip:** Cuando quedan ~**13.000** tokens hasta el límite, se dispara un agente que **resume** el hilo; `/compact` manual da más control.

**Eco nosotros:** [CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md), **D15**; con modelos locales, usar **umbrales proporcionales** al contexto real, no los números fijos del producto cloud.

---

## 6. `bypassPermissions` omite toda la puerta de seguridad (**Peligro**)

**Tip:** Modo que **auto-aprueba** todas las herramientas, incluidas destructivas (`rm`, `git push --force`, etc.). Solo en entornos **totalmente confiables** y aislados.

**Eco nosotros:** [ARCHITECTURE.md §2.3](ARCHITECTURE.md), **D5**; equivalencia en nuestro CLI debe estar **oculta tras flags** claros y advertencias; nunca por defecto.

---

## 7. Clasificador YOLO y comandos de alto riesgo en auto-modo

**Tip:** En modo automático, el clasificador en dos etapas **bloquea** patrones tipo `curl`, `wget`, `ssh`, `git`, `kubectl`, `aws`, etc.; para ejecutarlos hace falta **aprobación manual** o reglas **allow** explícitas.

**Eco nosotros:** [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md), **D17**; al implementar fast paths locales, alinear categorías con esta lista para no sorprender al usuario.

---

## 8. `MEMORY.md` con techo duro (**Atención**)

**Tip:** El índice se inyecta **siempre**; si supera ~**200 líneas** o ~**25 KB**, el exceso se **trunca** (en referencia sin aviso fuerte). El índice debe ser **sólo punteros**, no el cuerpo de la memoria.

**Eco nosotros:** [MEMORY_SYSTEM.md §3](MEMORY_SYSTEM.md); preferible **avisar** por UX cuando se acerque al límite.

---

## 9. Agentes personalizados en Markdown

**Tip:** Ficheros `.md` con **YAML** (tools, modelo, `permissionMode`, …); el cuerpo es el system prompt; carga automática.

**Eco nosotros:** [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md), **D19**, §2.13 en [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 10. Agente Verification en segundo plano

**Tip:** Tras implementaciones, un agente **Verification** emite veredicto estructurado (**PASS** / **FAIL** / **PARTIAL**), visible en terminal (p. ej. en rojo) — útil como **quality gate** en CI.

**Eco nosotros:** perfil opcional **v2+** ([AGENT_PROFILES.md](AGENT_PROFILES.md)); invocación vía tool **Agent** o pipeline externo que llame al binario con perfil restringido; no es MVP.

---

## Resumen eco Go

| Tip | Paquetes / decisiones |
|-----|------------------------|
| 1 | `internal/prompt` — orden de capas; `AGENTS.md` en CWD |
| 2–8 | `internal/memory`, `internal/session`, `internal/permissions`, `internal/classifier` — **D14**, **D15**, **D17** |
| 3–9–10 | `internal/agentprofile`, [CUSTOM_AGENTS.md](CUSTOM_AGENTS.md) — **D13**, **D19** |
| 4 | Precios vía config/proveedor; sin suposición “más rápido = gratis” |

---

## Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: 10 tips, severidad, enlaces internos + Costs; eco Go |
| 2026-04-07 | §4: enlace [COSTS.md](COSTS.md). |
