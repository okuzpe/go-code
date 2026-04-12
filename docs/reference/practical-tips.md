# Practical tips (visible product decisions) — reference and Go mapping

Summary of **ten behaviors** that in Claude Code are **wired into the code** (not just documentation tricks). Source analyzed: [Practical Tips — claude-code-explain](https://claude-code-explain.helmcode.com/tips). Each tip links to our in-depth documents that cover the same topic.

**Legend:** standard · **Attention** · **Danger** (by impact on cost, security, or data loss).

---

## 1. Repo rules at the top of the prompt

**Tip:** In the reference product, `CLAUDE.md` at the repo root is one of the **first** pieces injected into context and orients the whole session.

**Our mapping:** conceptual equivalent is `CLAUDE.md` at the project root (convention already cited in [memory-system.md](./memory-system.md) and [CLAUDE.md](../../goclaw/CLAUDE.md)). Keep **one source of truth** for team rules — do not duplicate in memory what belongs in the repo.

---

## 2. Memory across sessions

**Tip:** Persistence under paths like `~/.claude/projects/<slug>/memory/`; facts the user asks to remember **come back** in future sessions.

**Our mapping:** [memory-system.md](./memory-system.md), D14, `internal/memory`. Adjust paths to `~/.goclaw/memory/` in goclaw.

---

## 3. Explore agent with a cheap model

**Tip:** **Explore** uses **Haiku** (fast and cheap) for code searches; delegating there saves tokens versus using the main model for the same task.

**Our mapping:** [agent-profiles.md](./agent-profiles.md) + [CLAUDE.md](../../goclaw/CLAUDE.md); with **Ollama**, assign a **7B** model to the Explore profile and reserve the large one for the main loop ([local-models.md](./local-models.md)).

---

## 4. "Fast mode" ≠ different model (**Attention**)

**Tip:** `/fast` in the reference product does **not** change the model (e.g. it stays Opus): it raises **compute priority** and the **price per input token** (roughly **6×** in the analyzed explanation).

**Our mapping:** if you offer a "priority" mode over a paid API, document **explicitly** price vs model; locally (Ollama) the analogue is usually "faster" only because of queue/GPU, not surcharges — do not copy the `/fast` semantics without reading the provider docs.

Cost reference: [Costs — claude-code-explain](https://claude-code-explain.helmcode.com/costs); local summary: [costs.md](./costs.md).

---

## 5. Auto-compact at ~13K free tokens

**Tip:** When ~**13,000** tokens remain before the limit, an agent fires that **summarizes** the thread; manual `/compact` gives more control.

**Our mapping:** [context-compaction.md](./context-compaction.md), D15; with local models, use **proportional thresholds** based on the real context window, not the fixed numbers from the cloud product.

---

## 6. `bypassPermissions` skips the entire security gate (**Danger**)

**Tip:** Mode that **auto-approves** all tools, including destructive ones (`rm`, `git push --force`, etc.). Only for **fully trusted and isolated** environments.

**Our mapping:** [CLAUDE.md](../../goclaw/CLAUDE.md) (permissions), D5; any equivalent in our CLI must be **behind explicit flags** and warnings; never the default.

---

## 7. YOLO classifier and high-risk commands in auto mode

**Tip:** In automatic mode, the two-stage classifier **blocks** patterns like `curl`, `wget`, `ssh`, `git`, `kubectl`, `aws`, etc.; running them requires **manual approval** or explicit **allow** rules.

**Our mapping:** [yolo-classifier.md](./yolo-classifier.md), D17; when implementing local fast paths, align the categories with this list to avoid surprising the user.

---

## 8. `MEMORY.md` with a hard ceiling (**Attention**)

**Tip:** The index is **always injected**; if it exceeds ~**200 lines** or ~**25 KB**, the excess is **truncated** (in the reference product without a strong warning). The index must contain **only pointers**, not the full memory body.

**Our mapping:** [memory-system.md §3](./memory-system.md); it is preferable to **warn the user by UX** when approaching the limit.

---

## 9. Custom agents in Markdown

**Tip:** `.md` files with **YAML** frontmatter (tools, model, `permissionMode`, …); the body is the system prompt; loaded automatically.

**Our mapping:** [custom-agents.md](./custom-agents.md), D19, [CLAUDE.md](../../goclaw/CLAUDE.md).

---

## 10. Verification agent in the background

**Tip:** After implementations, a **Verification** agent emits a structured verdict (**PASS** / **FAIL** / **PARTIAL**), visible in the terminal (e.g. in red) — useful as a **quality gate** in CI.

**Our mapping:** the **`coordinator`** profile and `spawn_agent` already exist ([agent-profiles.md](./agent-profiles.md)); other "Team" topologies from the reference product are **not** in goclaw — invocation via binary with a restricted profile remains a valid pattern.

---

## Go mapping summary

| Tip | Packages / decisions |
|-----|----------------------|
| 1 | `internal/app` — prompt layer order; `CLAUDE.md` in CWD via `buildProjectContext` |
| 2–8 | `internal/memory`, `internal/session`, `internal/permissions`, `internal/permissions/risk.go` — D14, D15, D17 |
| 3–9–10 | `internal/agents`, [custom-agents.md](./custom-agents.md) — D13, D19 |
| 4 | Pricing via config/provider; no assumption "faster = free" |

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: 10 tips, severity levels, internal links + Costs; Go mapping |
| 2026-04-07 | §4: link to [costs.md](./costs.md) |
| 2026-04-12 | Translated from Spanish to English |
