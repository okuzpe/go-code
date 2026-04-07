# Clasificador de auto-modo (“YOLO Classifier”) — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.3](ARCHITECTURE.md) y **§2.4**. Referencia (terceros, análisis de Claude Code): [YOLO Classifier — claude-code-explain](https://claude-code-explain.helmcode.com/yolo-classifier).

**Por qué es capa esencial:** en modo automático el modelo puede encadenar muchas herramientas sin pausa humana. Un **monitor que no es el mismo razonamiento del agente** reduce **daño accidental, scope creep, inyección de prompt vía herramientas** y **acciones irreversibles** (deploy a prod, `rm -rf`, exfiltración, etc.).

Nombre “YOLO” aquí es **jerga del producto analizado**; en nuestros docs equivale a **clasificador de seguridad previo a ejecutar `tool_use` en auto-modo**.

---

## 1. Consulta lateral, no razonamiento inline

- El clasificador hace **otra llamada** al LLM con **system prompt dedicado** y un **transcript reducido**.
- No sustituye al usuario en modo interactivo normal; encaja cuando la política es **Auto** / **aprobar sin preguntar** salvo bloqueo.

**Eco Go:** `internal/classifier` (o subpaquete de `permissions`) que usa el mismo `internal/llm` con plantillas embebidas (`//go:embed`), no el system prompt del agente principal.

---

## 2. Flujo alto nivel (referencia)

| Paso | Comprobación |
|------|----------------|
| 1 | ¿Reglas **always-deny**? → **DENY** inmediato |
| 2 | ¿Reglas **always-allow**? → **ALLOW** inmediato |
| 3 | ¿**Fast paths** (sin clasificador)? → **ALLOW** sin API lateral |
| 4 | Si no → **clasificador en dos etapas** (XML) → ALLOW o DENY |

Orden completo integrado con permisos del producto analizado está resumido al final de este doc (§10).

---

## 3. Protocolo XML de dos etapas

Solo una forma explícita libera la acción (fail-closed):

```text
<!-- Permitido -->
<block>no</block>

<!-- Bloqueado -->
<block>yes</block><reason>una frase corta</reason>
```

- La respuesta **debe empezar** por `<block>` — sin preámbulo del modelo.
- **Etapa 1 (“fast”):** decisión corta; `stop_sequences` típico para cerrar en `</block>` (modo `both`); pocos tokens de salida (p. ej. 64; en modo `fast` hasta ~256).
- **Etapa 2 (“thinking”):** revisión con razonamiento; muchos más tokens de salida (p. ej. hasta ~4096); se dispara si etapa 1 bloquea o es **imparsable** (en modo `both`; en `fast` la imparsable → DENY).
- **Modos:** `both` (defecto), `fast` (solo etapa 1), `thinking` (solo etapa 2).

**Eco Go:** tipos `Stage1Result`, parser XML estricto; temperatura **0**; timeouts; tests con respuestas malformadas → DENY.

---

## 4. System prompt (estructura conceptual)

En referencia se ensambla con plantillas (base + reglas de intención + plantilla de permisos). Bloques conceptuales:

- **Base:** modelo de amenaza (prompt injection, scope creep, daño accidental); **solo** bloqueos de seguridad, no “calidad” ni eficiencia.
- **Reglas de intención usuario vs acción agente** (consentimiento explícito, escalada, etc.).
- **Plantilla BLOCK / ALLOW** (~**28** categorías BLOCK orientativas, ~**7** ALLOW) + contexto de entorno (dominios de confianza, etc.).

**Reglas de evaluación** (ejemplos de las 12 de referencia): acciones compuestas como un todo; escribir script y ejecutarlo = ejecutar comandos; `npm run deploy` = ejecutar el script; bloquear intentos de **desactivar el clasificador**; cada acción se juzga **por sí sola** (aprobaciones previas no arrastran); sesgo a bloquear en **infra compartida**.

Lista completa en el enlace helmcode; no duplicar aquí el catálogo propietario.

---

## 5. Config “autoMode” y fuentes de verdad

En referencia, `autoMode` en `settings.json` puede listar allow/soft_deny/environment — pero **`autoMode` desde ficheros del proyecto (p. ej. `.claude/settings.json`) se ignora** para evitar que un repo malicioso **inyecte** reglas que debiliten el clasificador.

**Eco Go:** reglas ampliadas solo desde **config del usuario** (`~/.config/assistant/…`) o flags explícitos; nunca aplicar ciegamente políticas de **solo proyecto** para auto-aprobación sin revisión.

---

## 6. Fast paths (evitar coste de API)

Orden conceptual (del más barato al más caro):

1. Tool devuelve **cadena vacía** en `ToClassifierInput` → “irrelevante para seguridad” → sin clasificador.
2. **Safety check** que no es “classifier-approvable” → ir directo a **ASK** humano.
3. Tool **requiere interacción** → prompt usuario.
4. PowerShell sin feature flag (ref.) → prompt usuario.
5. **acceptEdits:** simular política de edición; si pasaría en modo aceptar ediciones → ALLOW (excluye Agent/REPL en ref.).
6. **Allowlist de herramientas seguras** (Read, Grep, Glob, ciertas Task*, AskUserQuestion, Sleep, etc. en ref.) → ALLOW directo.

Write/Edit **no** suelen estar en la allowlist: pasan por acceptEdits o clasificador.

**Eco Go:** interfaz en cada herramienta:

```go
// ClassifierInput devuelve "" si la tool declara que no necesita clasificador lateral.
type Tool interface {
    ToClassifierInput(args any) string
}
```

---

## 7. Construcción del transcript (anti auto-influence)

**Incluir**

- Mensajes de **usuario:** solo bloques de **texto** (no imágenes ni tool results).
- Mensajes de **asistente:** solo bloques **`tool_use`** (lo que el modelo **hizo**, no lo que **dijo**).
- Comandos en cola si aplica.

**Excluir**

- **Texto libre del asistente** — para que no condicione al clasificador (“sesgo del narrador”).

Formato típico JSONL compacto por turno; reglas del proyecto pueden ir como prefijo cacheado aparte.

**Eco Go:** `session.BuildClassifierTranscript()` reutilizable por compactación; límites de tamaño; si transcript demasiado largo → en ref. CLI pregunta; headless puede **abortar**.

---

## 8. Patrones peligrosos y endurecimiento al entrar en auto-modo

- Listas de prefijos/comandos (bash, interpreters, `npm run`, `ssh`, `curl`, PowerShell, …) para **heurísticas** y para **retirar temporalmente** reglas “allow” del usuario que coincidan con patrones peligrosos al activar auto-modo (**strip**), restaurando al salir (**restore**).

**Eco Go:** tabla en `internal/permissions/patterns.go`; tests por plataforma (Unix vs Windows).

---

## 9. Contadores de denegación y fail-closed por defecto

| Concepto | Valor típico (ref.) |
|----------|---------------------|
| Denegaciones **consecutivas** max | 3 → volver a humano (CLI) o abortar (headless) |
| Denegaciones **totales** por sesión | 20 |
| Cualquier ALLOW | resetea consecutivas |

| Fallo | Comportamiento (ref.) |
|--------|------------------------|
| Error API clasificador | Por defecto **DENY** (“iron gate” cerrada); opción **ASK** si se abre la compuerta |
| Respuesta imparsable | Modo `both`: escalar a etapa 2; `fast` o etapa 2 falla → **DENY** |
| Cancelación usuario | DENY / bloqueado |

**Principios** (síntesis de los 10 de referencia): denegación por defecto; solo **`<block>no</block>`** explícito permite; **asistente texto fuera** del transcript; reglas peligrosas retiradas en auto; **settings de proyecto** no deben debilitar política global; dos etapas para falsos positivos; **caché** en prefijos repetidos (system + reglas + última acción) para coste.

---

## 10. Codificación por herramienta (`toAutoClassifierInput`)

Cada tool expone qué ve el clasificador: comando bash, ruta+contenido en edición, URL+prompt en fetch, query de búsqueda, delegación Agent con tipo y prompt, etc. Cadena vacía → salto del clasificador.

**Eco Go:** matriz tool → string en el **contrato de herramientas** (anexo o `TOOL_CONTRACT.md` cuando exista); misma función usada para logs auditables (sin datos sensibles sin mascarar).

---

## 11. Coste y modelo del clasificador

- Cada `tool_use` que no cae en fast path ⇒ **1–2 llamadas** extra con transcript potencialmente largo.
- Etapa 2 puede ser cara en tokens de salida.
- En referencia el modelo del clasificador suele alinearse con el del bucle principal (o override por config/env).

**Con Ollama:** segunda llamada al mismo host es viable; hay que valorar latencia y si un modelo **más pequeño** basta para etapa 1 (**D17**).

---

## 12. Eco Go (paquetes y fases)

| Pieza | Ubicación sugerida |
|-------|---------------------|
| Orquestación deny/ask/allow antes de tool | `internal/permissions` (ya previsto) |
| Llamada lateral + parser XML + etapas | `internal/classifier` **o** `internal/permissions/classifier` |
| Patrones peligrosos + strip/restore | `internal/permissions` + tests |
| Transcript seguro | `internal/session` helper compartido con observabilidad |
| Plantillas system | `embeds/classifier/*.txt` + `//go:embed` |

**Dependencias:** `classifier` → `llm`; `orchestrator` → `permissions` → opcionalmente `classifier`; evitar `classifier` → `orchestrator`.

**Roadmap alineado [ARCHITECTURE.md §4.4](ARCHITECTURE.md):**

| Fase | Clasificador |
|------|----------------|
| **MVP** | Sin API lateral: confirmación humana / modos conservadores (**D5**) |
| **v1** | Fast paths **locales** + patrones peligrosos + `ToClassifierInput` por tool; allowlist de tools de bajo riesgo |
| **v2+** | Dos etapas XML + denegaciones + iron gate + integración auto-modo (**D17**) |

---

## 13. Relación con otros docs

- **[RETRY_LOGIC.md](RETRY_LOGIC.md):** las llamadas al modelo del clasificador deben usar política de reintentos **acotada**; en fallos repetidos aplica el **iron gate** (§9), no un backoff ilimitado.
- **[COORDINATOR_MODE.md](COORDINATOR_MODE.md):** delegar a sub-agentes debe pasar por evaluación de intención (reglas de sub-agent en referencia).
- **[AGENT_PROFILES.md](AGENT_PROFILES.md):** modo `dontAsk` sin clasificador sólido es **peligroso**; alinear con Auto.
- **§2.4 shell:** el clasificador es **complemento** de validación sintáctica/sandbox, no sustituto.
- **[HOOKS.md](HOOKS.md):** `PreToolUse` / `PermissionRequest` pueden bloquear o mutar antes del pipeline YOLO; el **orden** hooks vs fast paths vs API lateral se fija en **D17 + D18** y en el pseudocódigo de permisos.

---

## 14. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: flujo, XML dos etapas, fast paths, transcript, fail-closed, eco Go, enlace helmcode §17 |
| 2026-04-07 | §13: interacción con [HOOKS.md](HOOKS.md) y D18 (orden de pipeline). |
| 2026-04-07 | §13: [RETRY_LOGIC.md](RETRY_LOGIC.md) y reintentos del clasificador. |
