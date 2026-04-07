# Coordinator Mode y Team/Swarm — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.11](ARCHITECTURE.md). Referencia conceptual (terceros, análisis del código de Claude Code): [Coordinator Mode — claude-code-explain](https://claude-code-explain.helmcode.com/coordinator-mode).

**Mensaje central:** en el producto analizado hay **dos sistemas multi-agente distintos**, no uno solo con dos nombres. Mezclarlos en diseño propio produce permisos incoherentes (p. ej. un “coordinador” que aún puede `Write`).

---

## 1. Dos topologías

| Aspecto | **Coordinator Mode** (hub-and-spoke) | **Team / Swarm** (peer-to-peer) |
|---------|--------------------------------------|----------------------------------|
| Topología | Un **coordinador** central; solo él habla con workers | Cualquier compañero puede mensajear a cualquier otro |
| Comunicación | Coordinador ↔ workers | Many-to-many |
| UI terminal | Sin UI especial en referencia | **tmux** (splits, colores); alternativa **iTerm2** (AppleScript) en macOS |
| Activación (ref.) | Flag de build + env `CLAUDE_CODE_COORDINATOR_MODE=1` | Tool **Agent** con parámetro **`team_name`** |
| Mejor encaje (ref.) | Tareas complejas con orquestación **estricta** | Trabajo colaborativo paralelo |

---

## 2. Coordinator Mode (detalle)

### 2.1 Restricción de seguridad clave

El **coordinador no accede al filesystem ni al shell**: no Read/Write/Edit/Bash directos. Toda mutación o lectura de código pasa por **workers** con toolbox completo. Es una capa de **razonamiento y delegación** sin superficie de “accidentalmente edité el repo”.

### 2.2 Herramientas del coordinador (referencia)

Solo un subconjunto **~4 herramientas** orientadas a orquestación, p. ej.:

- **Agent** — lanzar / enrutar workers  
- **SendMessage** — continuar un worker por id o enviar instrucciones  
- **TaskStop** — detener tareas  
- **SyntheticOutput** — salida estructurada cuando el producto lo requiere  

(Los workers conservan el conjunto completo salvo lo que el producto bloquee por política; en referencia el coordinador está fuertemente capado.)

### 2.3 Otros cambios respecto al modo normal

| Aspecto | Modo normal | Coordinator mode (ref.) |
|---------|-------------|-------------------------|
| System prompt | Prompt estándar | Prompt **específico de coordinador** |
| Acceso a ficheros / shell | Directo vía tools | **Solo vía workers** |

### 2.4 Flujo de trabajo en cuatro fases (referencia)

| Fase | Quién | Objetivo |
|------|--------|----------|
| 1. Research | Workers (**paralelo**) | Explorar código / contexto |
| 2. Synthesis | Coordinador | Leer hallazgos; **redactar spec** con rutas y **números de línea** concretos |
| 3. Implementation | Workers (**secuencial** por área de ficheros) | Aplicar cambios según spec; un worker a la vez por zona |
| 4. Verification | Workers | Comprobar que funciona; puede solaparse con impl en **áreas distintas** |

### 2.5 Resultado de worker → coordinador (formato de referencia)

Los resultados vuelven como mensajes **user-role** con XML encapsulado, p. ej.:

- `task-id`, `status` (`completed` \| `failed` \| `killed`)
- `summary`, `result` (salida detallada), `usage` (tokens u métricas)

**Eco Go:** tipo `WorkerNotification` + XML o JSON estable; el orquestador parsea y **reescribe** el estado interno sin pasar el transcript crudo del worker al siguiente turno del coordinador si la política lo evita.

### 2.6 Continue vs spawn (heurística de referencia)

| Situación | Acción | Razón |
|-----------|--------|--------|
| El worker ya exploró ficheros que hay que editar | **Continue** (`SendMessage` al mismo id) | Conserva contexto cargado |
| Research amplio pero implementación estrecha | **Spawn** nuevo | No arrastrar contexto innecesario |
| Reparar un fallo | **Continue** | Mantiene el error en contexto |
| Verificar código de otro worker | **Spawn** nuevo | Ojos frescos |
| El enfoque anterior fue erróneo | **Spawn** nuevo | Reinicio limpio |

### 2.7 Regla crítica: aislamiento del prompt del worker

Los workers **no ven** la conversación del coordinador con el usuario. Cada prompt de worker debe ser **autocontenido**: el coordinador debe volcar en la delegación las **rutas, líneas e instrucciones exactas**. Prohibido confiar en frases tipo “según tus hallazgos anteriores” — el worker no tiene esos hallazgos salvo que estén en **su** mensaje de tarea.

**Encaje con [AGENT_PROFILES.md §3](AGENT_PROFILES.md)** (inyectar en delegación vs cargar `CLAUDE.md` entero): misma filosofía de **prompt mínimo pero suficiente**.

---

## 3. Team / Swarm (detalle)

### 3.1 Estructura en disco (referencia, rutas ilustrativas bajo `~/.claude/`)

| Artefacto | Ruta típica (ref.) |
|-----------|-------------------|
| Config del equipo | `teams/{team-name}/config.json` |
| Lista de tareas | `tasks/{team-name}/` |
| Buzones (inboxes) | `teams/{team-name}/inboxes/{name}.json` |
| Rol fijo “team lead” | Nombre reservado p. ej. `team-lead` |

### 3.2 Flujo resumido

1. Team lead crea equipo (**TeamCreate**).  
2. Crea tareas (herramientas **Task***).  
3. Lanza compañeros (**Agent** con `team_name` + nombre).  
4. Asigna dueños (**TaskUpdate**, campo owner).  
5. Compañeros trabajan y cierran tareas.  
6. Mensajería con **SendMessage** (peer-to-peer).  
7. Apagado: **SendMessage** con tipo `shutdown_request` / respuesta.  
8. Limpieza **TeamDelete** cuando no queden compañeros activos.

### 3.3 Backends de “terminal” (referencia)

| Backend | Notas |
|---------|--------|
| **In-process** | Aislado con AsyncLocalStorage (en TS); en tests / headless |
| **tmux** | Proceso Claude Code separado por panel; splits y títulos |
| **iTerm2** | macOS, AppleScript para splits/pestañas |

Layouts (ref.): dentro de tmux existente → lead ~30% izquierda, equipo ~70% derecha (`main-vertical`); fuera de tmux → sesión nueva `claude-swarm`, ventana `swarm-view`, socket aislado, layout `tiled`.

### 3.4 SendMessage dual (referencia)

- **Coordinator Mode:** `SendMessage({ to: "<agent-id>", message: "..." })` continúa un worker.  
- **Team/Swarm:** mensaje a un compañero por nombre, o **broadcast** `to: "*"` (uso moderado).

**Enrutado (ref.):** prefijos tipo `bridge:<id>`, `uds:<path>`, id de agente registrado, `*` para broadcast a buzones, o nombre → escritura en mailbox de fichero.

### 3.5 Mailboxes (ficheros JSON + bloqueo)

- Mensajes JSON: `from`, `text`, `timestamp`, `read`, opcionales `color`, `summary`.  
- **proper-lockfile** (en referencia) para carreras entre escritores.  
- Poll ~**1 s** (hook UI React en referencia).  
- El poll también puede canalizar permisos, sandbox, shutdown, aprobación de plan, etc.

Inyección al contexto del compañero como XML, p. ej. `<teammate-message teammate_id="..." ...>`.

---

## 4. Tabla resumen Coordinator vs Team/Swarm

| Aspecto | Coordinator | Team/Swarm |
|---------|-------------|------------|
| Topología | Hub-and-spoke | Peer-to-peer |
| Mensajes | Coordinador ↔ workers | Cualquiera ↔ cualquiera |
| Tools del “centro” | Solo orquestación | Team lead con conjunto amplio (ref.) |
| Workers | Toolbox completo (sin Agent/SendMessage hacia fuera según política) | Toolbox + **SendMessage** entre pares |
| Entrega de trabajo | XML `task-notification` | Mailboxes + XML teammate-message |
| UI | Sin fuerza tmux | tmux / iTerm2 |

---

## 5. Eco Go (sin copiar el stack TS)

| Pieza referencia | Propuesta Go |
|------------------|--------------|
| Flag + env activador | `internal/config`: `CoordinatorMode bool`, validado en arranque |
| Perfil tool del rol | [AGENT_PROFILES.md](AGENT_PROFILES.md): **Coordinator** = allowlist explícita; **Worker** = General-Purpose o especializado |
| Workers aislados | Proceso hijo **o** goroutine + `session` fork por `agent_id`; **nunca** compartir slice de mensajes del coordinador |
| SendMessage / TaskStop | `internal/tools` + `orchestrator` enrutando a registro de workers vivos |
| Team/Swarm mailboxes | `internal/swarm` o `internal/team`: `Mailbox interface { Push, Poll }` con implementación **dir+lock** primero; luego opcional Redis/DB |
| tmux/iTerm2 | **Opcional y tardío**: en Windows priorizar **headless** + logs; tmux no es MVP |
| Polling 1 s | `time.Ticker` o eventos con `fsnotify` donde tenga sentido |

**Dependencias (alineado con [ARCHITECTURE.md §4.3](ARCHITECTURE.md)):** `coordinator` / `swarm` no deben importar `tools` de forma circular; el **orquestador** mantiene el mapa `agentID → run context`.

---

## 6. Roadmap alineado con ARCHITECTURE §4.4

| Fase | Qué cubrir |
|------|------------|
| MVP–v1 | Sin multi-agente; un solo bucle (quizá sub-agentes simples más adelante) |
| v2 | **Coordinator Mode mínimo** (opcional): env + tool allowlist + workers con sesión aislada; sin tmux obligatorio |
| v3+ | **Team/Swarm**: mailboxes, tareas compartidas, UI opcional; ver **D16** |

---

## 7. Relación con otros docs

- **[CONTEXT_COMPACTION.md](CONTEXT_COMPACTION.md):** cada worker tiene su propio presupuesto de contexto; el coordinador compacta **su** hilo, no el interno del worker.  
- **[MEMORY_SYSTEM.md](MEMORY_SYSTEM.md):** decisiones estables tras una sesión multi-agente pueden merecer `project` / `feedback`; no sustituyen specs en XML.  
- **§2.1 herramientas dedicadas:** los workers siguen D12; el coordinador no usa Bash para leer repo.
- **[YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md):** en auto-modo, los workers con toolbox completo deben pasar el mismo **gate** de seguridad que una sesión simple; la delegación no sustituye al clasificador.

---

## 8. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación: dos topologías, fases, mailboxes, continue/spawn, invariante “worker no ve al coordinador”, eco Go, enlace helmcode §16 |
