# Cloud billing modes — Go mapping

**Status in goclaw:** The **shipped CLI** uses **local Ollama only** — there is no per-token cloud bill from goclaw itself; cost is **hardware + electricity** (see §1). The “Anthropic vs Ollama” framing below is **conceptual** (comparison to other products and to a hypothetical future paid provider); it does not describe a selectable cloud provider in `settings.json` today.

Kept as a short companion to [practical-tips.md §4](./practical-tips.md) ("fast mode" pricing nuance); not merged into that file so [docs-map.md](../docs-map.md) can index cloud-cost topics separately.

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

- **goclaw today:** the supported runtime is **Ollama only**; there is no selectable cloud `provider` for billing. Cost levers are **hardware**, **which models you pull**, and optional **`task_models`** / per-profile **`model`** overrides (see [model-routing.md](../goclaw/model-routing.md), [agent-profiles.md](./agent-profiles.md)).
- For a **hypothetical** multi-provider fork: document in **config** which provider and model each profile uses, and expose tier/cost impact in the CLI/README (**D1**).

---

## 4. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created stub: D1, tips, profiles; docs-map |
| 2026-04-12 | Translated from Spanish to English |
| 2026-04-14 | Ollama-only CLI in status + §3 (real levers vs hypothetical multi-provider fork) |
