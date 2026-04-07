# Seguridad del shell (Bash) — referencia y eco Go

Profundidad ligada a [ARCHITECTURE.md §2.4](ARCHITECTURE.md). Explainer tercero: [Bash Security — claude-code-explain](https://claude-code-explain.helmcode.com/bash-security). Mapa: [DOCS_MAP.md](DOCS_MAP.md) (fila 21).

---

## 1. Alcance

En el producto analizado hay **varias capas**: validadores de comando, pipeline de permisos, sandbox (p. ej. seatbelt/bubblewrap en Unix), validación de rutas, saneado de entorno, y reglas de categoría (prompt injection vía resultados de herramientas).

**Nuestro MVP** no exige paridad con docenas de validadores: sí **defensa en profundidad mínima** alineada con **D4** (Windows), **D5** (modos) y [YOLO_CLASSIFIER.md](YOLO_CLASSIFIER.md) cuando exista auto-modo.

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
