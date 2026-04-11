# Skills (flujos reutilizables) — referencia y eco Go

**Status in goclaw:** [`goclaw/internal/skills`](../../goclaw/internal/skills) discovers and injects `SKILL.md` content; see [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). Below is **reference-product** detail.

Profundidad ligada a [CLAUDE.md](../../goclaw/CLAUDE.md) (skills runtime). Explainer tercero: [Skills — claude-code-explain](https://claude-code-explain.helmcode.com/skills). Mapa global: [docs-map.md](../docs-map.md) (fila 12).

---

## 1. Qué es un skill (referencia)

- Directorio o fichero con **`SKILL.md`**: frontmatter (nombre, descripción “use when…”, herramientas permitidas, opcionalmente hooks en sesión).
- El **cuerpo** Markdown instruye el flujo; invocación tipo **`/comando`** en productos con muchos slash commands.
- Encaja con [hooks.md](./hooks.md) cuando el frontmatter declara hooks de sesión (`PostToolUse`, etc.).

---

## 2. Eco Go

| Pieza | Ubicación |
|-------|-----------|
| Descubrimiento `**/SKILL.md` | `internal/skills` (v3 según [roadmap.md](../goclaw/roadmap.md)) |
| Inyección en prompt | `internal/prompt` o orquestador al activar skill |
| Comandos slash | Opcional: plugins [plugins.md](./plugins.md) o CLI propio; **no MVP** |

**Roadmap:** **v3** junto a `internal/skills` y plugins; contrato vigente: [CLAUDE.md](../../goclaw/CLAUDE.md) y las secciones de este archivo.

---

## 3. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación stub: formato, hooks, roadmap v3; DOCS_MAP |
