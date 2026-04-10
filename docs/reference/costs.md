# Costes y modos de facturación (referencia cloud) — eco Go

**Status in goclaw:** Billing is **provider-dependent** (Anthropic vs Ollama/local). Kept as a short companion to [practical-tips.md §4](./practical-tips.md) (“fast mode” pricing nuance); not merged into that file so [docs-map.md](../docs-map.md) can index cloud-cost topics separately.

Mapa: [docs-map.md](../docs-map.md). Explainer tercero: [Costs — claude-code-explain](https://claude-code-explain.helmcode.com/costs).

---

## 1. Cuándo importa

Solo si **D1** incluye API de pago (Anthropic, OpenAI, etc.). Con **Ollama/local** ([local-models.md](./local-models.md)) el modelo de coste es **hardware + electricidad**, no precio por token.

---

## 2. Patrones del producto analizado

- Varios **modelos** con precios distintos; modo **“fast”** puede implicar **mayor tarifa por token** sin cambiar de modelo — ver [practical-tips.md §4](./practical-tips.md).
- Delegar búsquedas al agente **Explore** con modelo barato ahorra frente al bucle principal — [agent-profiles.md](./agent-profiles.md).

---

## 3. Eco Go

- Documentar en **config** qué proveedor y modelo usa cada perfil.
- Si hay “prioridad” o tier premium: **exponer** en CLI/README el impacto en coste (**D1**).

---

## 4. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación stub: D1, tips, perfiles; docs-map |
