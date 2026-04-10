# Seguridad del shell (Bash) — referencia y eco Go

Profundidad ligada a [ARCHITECTURE_LEGACY_ES.md §2.4](../archive/architecture-legacy-es.md). Explainer tercero: [Bash Security — claude-code-explain](https://claude-code-explain.helmcode.com/bash-security). Mapa: [docs-map.md](../docs-map.md) (fila 21).

---

## 1. Alcance

En el producto analizado hay **varias capas**: validadores de comando, pipeline de permisos, sandbox (p. ej. seatbelt/bubblewrap en Unix), validación de rutas, saneado de entorno, y reglas de categoría (prompt injection vía resultados de herramientas).

**Nuestro MVP** no exige paridad con docenas de validadores: sí **defensa en profundidad mínima** alineada con **D4** (Windows), **D5** (modos) y [YOLO_CLASSIFIER.md](./yolo-classifier.md) cuando exista auto-modo.

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
| Sandbox OS | Fase tardía; Windows es el caso duro |
| Clasificador | v2+: patrón CC para `curl`/`ssh`/… en auto-modo |

---

## 3. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación stub: capas, D4/D5, enlace helmcode; DOCS_MAP |
| 2026-04-08 | §1.1 goclaw: allowlist + metacaracteres, enlace a `bash.go` y TOOL_CONTRACT |
