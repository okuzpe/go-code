# Cloud billing modes — Go mapping

**Status in goclaw:** Billing is **provider-dependent** (Anthropic vs Ollama/local). Kept as a short companion to [practical-tips.md §4](./practical-tips.md) ("fast mode" pricing nuance); not merged into that file so [docs-map.md](../docs-map.md) can index cloud-cost topics separately.

Map: [docs-map.md](../docs-map.md). Third-party explainer: [Costs — claude-code-explain](https://claude-code-explain.helmcode.com/costs).

---

## 1. When it matters

Only if **D1** includes a paid API (Anthropic, OpenAI, etc.). With **Ollama/local** ([local-models.md](./local-models.md)) the cost model is **hardware + electricity**, not per-token pricing.

---

## 2. Patterns from the reference product

- Several **models** at different prices; **"fast"** mode may imply a **higher per-token rate** without switching models — see [practical-tips.md §4](./practical-tips.md).
- Delegating searches to the **Explore** agent with a cheaper model saves cost vs. the main loop — [agent-profiles.md](./agent-profiles.md).

---

## 3. Go mapping

- Document in **config** which provider and model each profile uses.
- If there is a priority tier or premium tier: **expose** the cost impact in the CLI/README (**D1**).

---

## 4. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created stub: D1, tips, profiles; docs-map |
| 2026-04-12 | Translated from Spanish to English |
