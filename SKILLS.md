# Skills (flujos reutilizables) — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.9](ARCHITECTURE.md). Explainer tercero: [Skills — claude-code-explain](https://claude-code-explain.helmcode.com/skills). Mapa global: [DOCS_MAP.md](DOCS_MAP.md) (fila 12).

---

## 1. Qué es un skill (referencia)

- Directorio o fichero con **`SKILL.md`**: frontmatter (nombre, descripción “use when…”, herramientas permitidas, opcionalmente hooks en sesión).
- El **cuerpo** Markdown instruye el flujo; invocación tipo **`/comando`** en productos con muchos slash commands.
- Encaja con [HOOKS.md](HOOKS.md) cuando el frontmatter declara hooks de sesión (`PostToolUse`, etc.).

---

## 2. Eco Go

| Pieza | Ubicación |
|-------|-----------|
| Descubrimiento `**/SKILL.md` | `internal/skills` (v3 según [ARCHITECTURE.md §4.4](ARCHITECTURE.md)) |
| Inyección en prompt | `internal/prompt` o orquestador al activar skill |
| Comandos slash | Opcional: plugins [PLUGINS.md](PLUGINS.md) o CLI propio; **no MVP** |

**Roadmap:** **v3** en §4.4 junto a `skills/` en el árbol; hasta entonces §2.9 en ARCHITECTURE basta como contrato.

---

## 3. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación stub: formato, hooks, roadmap v3; DOCS_MAP |
