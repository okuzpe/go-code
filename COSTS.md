# Costes y modos de facturación (referencia cloud) — eco Go

Mapa: [DOCS_MAP.md](DOCS_MAP.md) (fila 07). Explainer tercero: [Costs — claude-code-explain](https://claude-code-explain.helmcode.com/costs).

---

## 1. Cuándo importa

Solo si **D1** incluye API de pago (Anthropic, OpenAI, etc.). Con **Ollama/local** ([LOCAL_MODELS.md](LOCAL_MODELS.md)) el modelo de coste es **hardware + electricidad**, no precio por token.

---

## 2. Patrones del producto analizado

- Varios **modelos** con precios distintos; modo **“fast”** puede implicar **mayor tarifa por token** sin cambiar de modelo — ver [PRACTICAL_TIPS.md §4](PRACTICAL_TIPS.md).
- Delegar búsquedas al agente **Explore** con modelo barato ahorra frente al bucle principal — [AGENT_PROFILES.md](AGENT_PROFILES.md).

---

## 3. Eco Go

- Documentar en **config** qué proveedor y modelo usa cada perfil.
- Si hay “prioridad” o tier premium: **exponer** en CLI/README el impacto en coste (**D1**).

---

## 4. Changelog

| Fecha | Cambio |
|-------|--------|
| 2026-04-07 | Creación stub: D1, tips, perfiles; DOCS_MAP |
