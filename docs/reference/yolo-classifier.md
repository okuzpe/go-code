# Auto-mode classifier ("YOLO Classifier") — reference and Go mapping

**Status in goclaw:** **Rule-based risk scoring** (0–100) and optional `yolo_threshold` auto-approval live in [`goclaw/internal/permissions/risk.go`](../../goclaw/internal/permissions/risk.go) — not a separate lateral LLM call as in the reference product. See **D17** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md).

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (D17 risk) and **§2.4**. Reference (third-party, Claude Code analysis): [YOLO Classifier — claude-code-explain](https://claude-code-explain.helmcode.com/yolo-classifier).

**Why this is an essential layer:** in automatic mode the model can chain many tools without a human pause. A **monitor that is not the agent's own reasoning** reduces **accidental damage, scope creep, prompt injection via tools**, and **irreversible actions** (prod deploy, `rm -rf`, exfiltration, etc.).

The name "YOLO" here is **jargon from the analyzed product**; in our docs it means the **security classifier that runs before executing `tool_use` in auto mode**.

---

## 1. Lateral call, not inline reasoning

- The classifier makes **another call** to the LLM with a **dedicated system prompt** and a **reduced transcript**.
- It does not replace the user in normal interactive mode; it fits when the policy is **Auto** / approve without asking, except for blocks.

**Go mapping:** `internal/classifier` (or sub-package of `permissions`) that uses the same `internal/llm` with embedded templates (`//go:embed`), not the main agent's system prompt.

---

## 2. High-level flow (reference)

| Step | Check |
|------|-------|
| 1 | **Always-deny** rules? → immediate **DENY** |
| 2 | **Always-allow** rules? → immediate **ALLOW** |
| 3 | **Fast paths** (no classifier)? → **ALLOW** without lateral API call |
| 4 | Otherwise → **two-stage classifier** (XML) → ALLOW or DENY |

Complete order integrated with the analyzed product's permissions is summarized at the end of this doc (§10).

---

## 3. Two-stage XML protocol

Only one explicit form releases the action (fail-closed):

```text
<!-- Allowed -->
<block>no</block>

<!-- Blocked -->
<block>yes</block><reason>a short phrase</reason>
```

- The response **must start** with `<block>` — no model preamble.
- **Stage 1 ("fast"):** short decision; typical `stop_sequences` to close at `</block>` (mode `both`); few output tokens (e.g. 64; in `fast` mode up to ~256).
- **Stage 2 ("thinking"):** review with reasoning; many more output tokens (e.g. up to ~4096); fires if stage 1 blocks or is **unparseable** (in `both` mode; in `fast` mode an unparseable → DENY).
- **Modes:** `both` (default), `fast` (stage 1 only), `thinking` (stage 2 only).

**Go mapping:** `Stage1Result` types, strict XML parser; temperature **0**; timeouts; tests with malformed responses → DENY.

---

## 4. System prompt (conceptual structure)

In the reference it is assembled from templates (base + intent rules + permissions template). Conceptual blocks:

- **Base:** threat model (prompt injection, scope creep, accidental damage); **only** security blocks, not "quality" or efficiency.
- **User-intent vs agent-action rules** (explicit consent, escalation, etc.).
- **BLOCK / ALLOW template** (~**28** illustrative BLOCK categories, ~**7** ALLOW) + environment context (trusted domains, etc.).

**Evaluation rules** (examples from the 12 in the reference): compound actions evaluated as a whole; writing a script and running it = running commands; `npm run deploy` = running the script; block attempts to **disable the classifier**; each action judged **on its own** (prior approvals do not carry over); bias toward blocking on **shared infrastructure**.

Full list at the helmcode link; do not duplicate the proprietary catalog here.

---

## 5. "autoMode" config and sources of truth

In the reference, `autoMode` in `settings.json` can list allow/soft_deny/environment — but **`autoMode` from project files (e.g. `.claude/settings.json`) is ignored** to prevent a malicious repo from **injecting** rules that weaken the classifier.

**Go mapping:** extended rules only from **user config** (`~/.goclaw/…`) or explicit flags; never blindly apply **project-only** policies for auto-approval without review.

---

## 6. Fast paths (avoiding API cost)

Conceptual order (cheapest to most expensive):

1. Tool returns **empty string** in `ToClassifierInput` → "irrelevant to security" → no classifier.
2. **Safety check** that is not "classifier-approvable" → go directly to **human ASK**.
3. Tool **requires interaction** → user prompt.
4. PowerShell without feature flag (ref.) → user prompt.
5. **acceptEdits:** simulate edit policy; if it would pass in accept-edits mode → ALLOW (excludes Agent/REPL in ref.).
6. **Safe tool allowlist** (Read, Grep, Glob, certain Task*, AskUserQuestion, Sleep, etc. in ref.) → direct ALLOW.

Write/Edit are usually **not** on the allowlist: they go through acceptEdits or the classifier.

**Go mapping:** interface on each tool:

```go
// ClassifierInput returns "" if the tool declares it does not need a lateral classifier call.
type Tool interface {
    ToClassifierInput(args any) string
}
```

---

## 7. Transcript construction (anti-self-influence)

**Include:**

- **User** messages: text blocks only (no images or tool results).
- **Assistant** messages: `tool_use` blocks only (what the model **did**, not what it **said**).
- Queued commands if applicable.

**Exclude:**

- **Assistant free text** — to prevent the classifier from being biased by the narrator's framing.

Typical compact JSONL format per turn; project rules can go as a separately cached prefix.

**Go mapping:** `session.BuildClassifierTranscript()` reusable for compaction; size limits; if transcript is too long → in ref. CLI asks; headless can **abort**.

---

## 8. Dangerous patterns and hardening on entering auto mode

- Lists of prefixes/commands (bash, interpreters, `npm run`, `ssh`, `curl`, PowerShell, …) for **heuristics** and for **temporarily removing** user "allow" rules that match dangerous patterns when entering auto mode (**strip**), restoring them on exit (**restore**).

**Go mapping:** table in `internal/permissions/patterns.go`; tests per platform (Unix vs Windows).

---

## 9. Denial counters and fail-closed default

| Concept | Typical value (ref.) |
|---------|----------------------|
| Max **consecutive** denials | 3 → return to human (CLI) or abort (headless) |
| Max **total** denials per session | 20 |
| Any ALLOW | resets consecutive counter |

| Failure | Behavior (ref.) |
|---------|-----------------|
| Classifier API error | Default **DENY** ("iron gate" closed); optional **ASK** if gate opens |
| Unparseable response | Mode `both`: escalate to stage 2; `fast` or stage 2 fail → **DENY** |
| User cancellation | DENY / blocked |

**Principles** (synthesis of the 10 in the reference): deny by default; only explicit **`<block>no</block>`** permits; **assistant text outside** the transcript; dangerous rules removed in auto; **project settings** must not weaken global policy; two stages for false positives; **cache** on repeated prefixes (system + rules + last action) for cost.

---

## 10. Per-tool encoding (`toAutoClassifierInput`)

Each tool exposes what the classifier sees: bash command, path+content for edits, URL+prompt for fetch, search query, Agent delegation with type and prompt, etc. Empty string → classifier skip.

**Go mapping:** tool → string matrix in the **tool contract** (`tool-contract.md`); same function used for auditable logs (without unmasked sensitive data).

---

## 11. Classifier cost and model

- Each `tool_use` that does not hit a fast path ⇒ **1–2 extra calls** with a potentially long transcript.
- Stage 2 can be expensive in output tokens.
- In the reference the classifier model usually aligns with the main loop model (or config/env override).

**With Ollama:** a second call to the same host is viable; evaluate latency and whether a **smaller model** suffices for stage 1 (**D17**).

---

## 12. Go mapping (packages and phases)

| Piece | Suggested location |
|-------|--------------------|
| Orchestrate deny/ask/allow before tool | `internal/permissions` (already planned) |
| Lateral call + XML parser + stages | `internal/classifier` **or** `internal/permissions/classifier` |
| Dangerous patterns + strip/restore | `internal/permissions` + tests |
| Safe transcript | `internal/session` helper shared with observability |
| System templates | `embeds/classifier/*.txt` + `//go:embed` |

**Dependencies:** `classifier` → `llm`; `orchestrator` → `permissions` → optionally `classifier`; avoid `classifier` → `orchestrator`.

**Aligned roadmap [roadmap.md](../goclaw/roadmap.md):**

| Area | Classifier |
|------|------------|
| **goclaw (shipped)** | **D17** — local rules + 0–100 scoring in `internal/permissions/risk.go`; `yolo_threshold` (default off); auto-approve reads at threshold 0; coexists with **D5** (modes) and **D18** hooks |
| **Reference (analyzed product)** | Two-stage XML LLM classifier, iron gate, caches — different implementation |
| **Not implemented in goclaw** | Lateral call to a dedicated "classifier" LLM as in the reference |

---

## 13. Relation to other docs

- **[retry-logic.md](./retry-logic.md):** classifier model calls must use a **bounded** retry policy; on repeated failures the **iron gate** applies (§9), not unlimited backoff.
- **[coordinator-mode.md](./coordinator-mode.md):** delegating to sub-agents must pass intent evaluation (sub-agent rules in reference).
- **[agent-profiles.md](./agent-profiles.md):** `dontAsk` mode without a solid classifier is **dangerous**; align with Auto.
- **§2.4 shell:** the classifier is a **complement** to syntactic/sandbox validation, not a substitute.
- **[hooks.md](./hooks.md):** `PreToolUse` / `PermissionRequest` can block or mutate before the YOLO pipeline; the **order** of hooks vs fast paths vs lateral API is fixed in **D17 + D18** and the permissions pseudocode.

---

## 14. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: flow, two-stage XML, fast paths, transcript, fail-closed, Go mapping, helmcode §17 link |
| 2026-04-07 | §13: interaction with [hooks.md](./hooks.md) and D18 (pipeline order) |
| 2026-04-07 | §13: [retry-logic.md](./retry-logic.md) and classifier retries |
| 2026-04-12 | Translated from Spanish to English |
