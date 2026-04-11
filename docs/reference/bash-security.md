# Seguridad del shell (Bash) — referencia y eco Go

Profundidad ligada a [CLAUDE.md](../../goclaw/CLAUDE.md) (bash / permissions). Explainer tercero: [Bash Security — claude-code-explain](https://claude-code-explain.helmcode.com/bash-security). Mapa: [docs-map.md](../docs-map.md) (fila 21).

---

## 1. Alcance

En el producto analizado hay **varias capas**: validadores de comando, pipeline de permisos, sandbox (p. ej. seatbelt/bubblewrap en Unix), validación de rutas, saneado de entorno, y reglas de categoría (prompt injection vía resultados de herramientas).

**goclaw hoy** no intenta paridad con docenas de validadores del producto de referencia: sí **defensa en profundidad mínima** alineada con **D4** (Windows), **D5** (modos) y [yolo-classifier.md](./yolo-classifier.md) cuando el auto-modo / **D17** están activos.

### 1.1 Implementación **goclaw** (hoy)

El tool `bash` en [`goclaw/internal/tools/bash.go`](../../goclaw/internal/tools/bash.go) combina:

- **Allowlist** de binarios y subcomandos `git` (ver código).
- **Escaneo de sintaxis shell** (`rejectShellMetacharacters`): bloquea tuberías (`|`), separadores (`;`, `&&`), redirecciones (`>`, `<`), subshells `(...)`, sustitución `$(...)`, backticks y `&` fuera de comillas (las URLs con `&` deben ir entre comillas). Así se evita que un primer token allowlist (p. ej. `curl`) ejecute binarios no listados vía `| sh`.
- **Permisos** [README — Permissions](../../goclaw/README.md): modo `ask` pide confirmación en stderr antes de ejecutar.

Detalle de contrato: [tool-contract.md](./tool-contract.md) fila `bash`.

---

## 2. Eco Go (orientativo)

| Capa | Notas |
|------|--------|
| Parseo / tokenización | Limitar metacaracteres, profundidad de pipelines, denies obvios (`rm -rf /`, etc.) |
| Permisos | Siempre antes de `exec`; ver §2.3 |
| Rutas | Trabajo dentro del workspace; denegar salidas si procede |
| Entorno | Subprocess con env reducido (sin secretos heredados sin filtrar) |
| Sandbox OS | No implementado (OS-level); Windows es el caso duro |
| Clasificador | **D17** en goclaw: reglas de riesgo + umbral; patrón distinto al clasificador LLM del producto de referencia |

---

## 3. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación stub: capas, D4/D5, enlace helmcode; DOCS_MAP |
| 2026-04-08 | §1.1 goclaw: allowlist + metacaracteres, enlace a `bash.go` y [tool-contract.md](./tool-contract.md) |
