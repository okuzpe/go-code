# OpenClaw — Gateway, daemon y canales (profundidad)

**GoClaw:** Shipped behavior → [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). Navigation hub → [architecture.md](../architecture.md).

Documento de apoyo a [architecture.md](../architecture.md) y [openclaw-reference.md](./openclaw-reference.md). Describe la pieza más **distintiva** de OpenClaw respecto a un “CLI agente” mínimo en Go: el **Gateway** como plano de control y la multiplexación de **canales** de entrada/salida.

**Nota:** en este workspace **no** hay clon de `openclaw/`; rutas de código → [GitHub openclaw/openclaw](https://github.com/openclaw/openclaw).

---

## 1. Rol del Gateway

Según la documentación del propio proyecto ([docs/index.md](https://github.com/openclaw/openclaw/blob/main/docs/index.md)):

- Un único proceso **Gateway** es la fuente de verdad para **sesiones**, **enrutado** y **conexiones de canal**.
- Los chat apps y plugins alimentan el Gateway; desde ahí se atienden agente (**Pi** en su terminología), CLI, Web Control UI y nodos móviles.

En el código upstream, conviven aproximadamente:

- [`src/gateway/`](https://github.com/openclaw/openclaw/tree/main/src/gateway) — protocolo, servidor, métodos expuestos.
- [`src/daemon/`](https://github.com/openclaw/openclaw/tree/main/src/daemon) — ejecución como servicio de usuario (p.ej. tras `openclaw onboard --install-daemon`).
- [`src/cli/gateway-cli/`](https://github.com/openclaw/openclaw/tree/main/src/cli/gateway-cli) y programas relacionados — superficie de comando para levantar o administrar el gateway.

---

## 2. Canales (`src/channels/`)

- Los **canales** encapsulan transporte hacia Discord, Telegram, Slack, etc.
- Hay subdirectorios para **plugins**, **transport**, **allowlists**, **web** y contratos de acciones/outbound.

**Para nuestro asistente en Go (fase inicial)**

- Podemos modelar un único “canal” **Terminal REPL** (stdin/stdout) que implemente la misma **idea** de “superficie de entrada” sin implementar el grafo completo de OpenClaw.
- Si más adelante queremos WhatsApp/Telegram, el diseño de OpenClaw sugiere: **adaptador por canal** + políticas de allowlist + cola de mensajes, en lugar de mezclar lógica de red con el núcleo del agente.

---

## 3. Enrutado (`src/routing/`)

OpenClaw enfatiza **multi-agent routing** y sesiones aisladas por agente, workspace o remitente (tarjetas en docs/index).

**Eco Go**

- Tip: mantener **routing** como módulo pequeño: `SessionID → cola de mensajes` y reglas de qué herramientas y memoria aplican, aunque al principio solo exista una sesión interactiva.

---

## 4. Web Control UI y `ui/`

- El repositorio upstream incluye [`ui/`](https://github.com/openclaw/openclaw/tree/main/ui) para el dashboard en navegador (chat, config, sesiones).

**Eco Go**

- Opcional y tardío: API HTTP mínima + frontend, o TUI en terminal únicamente.

---

## 5. Pairing y nodos (`src/pairing/`, `src/node-host/`)

- Flujos para emparejar dispositivos móviles con el Gateway.

**Eco Go**

- Fuera de alcance hasta definir explícitamente “asistente móvil + escritorio”.

---

## 6. Resumen: qué nos llevamos para decidir en Go

| Concepto OpenClaw | ¿Replicar en v1 Go? | Nota |
|-------------------|---------------------|------|
| Gateway persistente + daemon | Opcional | Útil si queremos “siempre encendido” y varios clientes |
| Multicanal (10+ apps) | No en v1 | Diseñar adapters detrás de una interfaz |
| Allowlists por canal | Más tarde | Misma idea que allowlist de hosts para `web_fetch` |
| Web Control UI | No en v1 | REPL/TUI primero |

---

## 7. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación del documento. |
| 2026-04-07 | Enlaces a GitHub (sin `openclaw/` local). |
