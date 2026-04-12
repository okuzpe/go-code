# Context and compaction — reference and Go mapping

**Status in goclaw:** Session compaction uses a **token heuristic** and configurable threshold — [`goclaw/internal/orchestrator/compaction.go`](../../goclaw/internal/orchestrator/compaction.go), **D15** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md). Numbers below are still calibrated against the **reference product**, not hard limits in Go.

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (D15 compaction). Conceptual reference (third-party analysis of Claude Code): [Context & Compaction — claude-code-explain](https://claude-code-explain.helmcode.com/context-compaction).

The concrete numbers on this page are **from the analyzed product**, not commitments of our binary: they serve to **calibrate** implementation and configuration flags (see [CLAUDE.md](../../goclaw/CLAUDE.md) D15).

---

## 1. Why this fits in our documents

| Topic (see [CLAUDE.md](../../goclaw/CLAUDE.md)) | Relation |
|-------------------------------------------------|----------|
| Context limits / LLM client (**D15**) | Compaction is policy **around** that client + history in `session`. |
| Micro vs. strong compaction (this doc §3–§4) | Micro-compaction vs. strong compaction; optional re-injection. |
| Agent profiles | Less injected context ⇒ less window pressure; does not replace pruning **tool results**. |
| Disk memory (**D13**) | After aggressive summarization, `MEMORY.md` and memory files remain the **stable** layer outside the thread. |
| `session` vs `llm` | The component that compacts with **another model call** is usually `orchestrator` or a worker, not the pure-state package. |
| Retries / [retry-logic.md](./retry-logic.md) (**D22**) | That extra call inherits the same **retry/backoff** policy, with bounded cancellation. |

---

## 2. Context window (analyzed reference)

| Context | Tokens (ref.) | Note |
|---------|---------------|------|
| Default | ~200,000 | "Default" in the reference product |
| "1M" models | ~1,000,000 | E.g. Sonnet 4.x / Opus 4.6 families with extended mode |
| Override | Custom | In reference: e.g. a variable like `CLAUDE_CODE_MAX_CONTEXT_TOKENS` |

**Go mapping:** read the effective limit from the provider (API) or local runtime (Ollama: context of the loaded model, much smaller than 200K in practice — see [local-models.md](./local-models.md)) and optionally allow override in config.

---

## 3. Auto-compaction (strong compaction)

Flow described in the reference:

1. **Threshold:** ~**13,000** free tokens remain before the ceiling → automatic compaction fires.
2. **Sub-agent / fork:** a process reads the full history and generates a **compressed summary**.
3. **Replacement:** the thread seen by the main model becomes that summary (not the full history).
4. **Post-compact budget:** ~**50,000** tokens to "recover" useful context; in the reference there are limits like **max 5 files**, **~5,000 tokens per file** (e.g. skills and key files).
5. **Circuit breaker:** if compaction **fails 3 times in a row**, it stops retrying to avoid loops.

**Go mapping:** async tasks with `context.Context`, same `llm` client but with a "summarizer" system prompt, consecutive-failure metric, and explicit re-injection policy (read from disk under budget rather than trusting the model to "remember" the repo).

**Manual compaction:** the reference product has a **`/compact`** command before hitting the limit; this usually produces more intentional summaries than a last-minute auto-compact. Go equivalent: REPL command or flag that invokes the same pipeline with confirmation.

---

## 4. Micro-compaction (inline, during the session)

Goal: **not** waiting for the global threshold; reduces noise from **old tool results**.

- When a `tool_result` "ages" (heuristic: age in history or turns since last use), its content is replaced by a short marker, e.g. **`[Old tool result content cleared]`** (illustrative text from the analyzed pattern).
- Tools covered in the reference for this treatment include among others: **Read, Bash, Grep, Glob, WebSearch, WebFetch, Edit, Write**.

**Images:** the reference uses a conservative fixed estimate (**~2,000 tokens** per image) for the budget even if the file differs.

**Go mapping:** implement in `internal/session` or `internal/contextwindow`: message queue with per-block metadata (`ToolName`, `tokensEstimate`, age); `MicroCompact(messages)` function before each `Complete`. Be careful **not** to clear results the last user turn still needs (heuristic or explicit "pinning" in v2).

---

## 5. Model output limits (reference)

Reference values from the analyzed product (useful for sizing `max_tokens` per phase):

| Mode | Max output tokens (ref.) |
|------|--------------------------|
| Standard response | ~32,000 |
| Capped (slot reservation) | ~8,000 |
| Scaled / recovery after errors | ~64,000 |
| **Compaction agent** output (summary) | ~20,000 |

**Go mapping:** parameterize by call type (`TurnUser`, `TurnCompaction`, `TurnRecovery`) in `internal/llm`.

---

## 6. What not to conflate

- **Compaction** ≠ **persistent memory** ([memory-system.md](./memory-system.md)): the summary lives in the thread; disk memory prevents losing stable facts when the thread is summarized.
- **Micro-compaction** ≠ **truncating on-disk logs**: only the **in-flight message** to the API.
- Numbers 13K / 50K / 32K are **reference**: with Ollama 7B the real ceiling can be 8K–32K depending on the model; thresholds should be a **fraction of the configured limit** or per-provider tables (D15).
- **Multi-agent:** the **coordinator** thread and each **worker** thread are independent; compacting one does not replace the others' history — see [coordinator-mode.md §2.7](./coordinator-mode.md) (worker isolation).

---

## 7. goclaw today vs. optional improvements

| Area | Compaction |
|------|------------|
| **goclaw (shipped)** | Token heuristic + configurable threshold, recent-turn **tail** preserved, phase 1 that clears old `tool_results`, `/compact`, **`llm_compaction`** + **`compaction_model`** option — see [`compaction.go`](../../goclaw/internal/orchestrator/compaction.go) and **D15** in [`CLAUDE.md`](../../goclaw/CLAUDE.md) |
| **Reference** | Aggressive micro-compact, post-compact budgets of tens of thousands of tokens, etc. — calibration, not a requirement |
| **Not implemented / future** | Automatic file re-injection under budget as in some reference products; see [roadmap.md](../goclaw/roadmap.md) if prioritized |

---

## 8. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: window, auto/micro-compact, output limits, Go mapping, helmcode §06 link |
| 2026-04-07 | §6: multi-agent (separate threads) → [coordinator-mode.md](./coordinator-mode.md) |
| 2026-04-07 | §1: table + [retry-logic.md](./retry-logic.md) (compaction sub-call) |
| 2026-04-12 | Translated from Spanish to English |
