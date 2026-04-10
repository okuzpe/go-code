# Retry Logic

**Status in goclaw:** Implemented — [`goclaw/internal/llm/retry.go`](../../goclaw/internal/llm/retry.go); decision **D22** in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md).

goclaw retries LLM HTTP requests on transient failures using exponential backoff. The retry budget is **per call** — each HTTP request to the model gets its own independent counter. A slow or rate-limited stream does not consume the budget for subsequent calls.

---

## Parameters

| Parameter | Value |
|-----------|-------|
| Maximum attempts per call | **10** |
| Base delay | **500 ms** (doubles each attempt) |
| Maximum delay per wait | **5 minutes** |

---

## Retry Conditions

**Retried with backoff:**
- HTTP `429` Too Many Requests
- HTTP `503` Service Unavailable
- HTTP `504` Gateway Timeout
- Transient network errors (connection refused, timeout, EOF before headers)

**Not retried (treated as permanent):**
- HTTP `401` Unauthorized, `403` Forbidden
- HTTP `400` Bad Request and other `4xx` client errors
- Payload or serialization errors (fix the request, not the retry count)

---

## Retry-After Header

When the provider returns a `Retry-After` header (integer seconds), goclaw uses that value instead of the computed backoff. The value is clamped to the 5-minute ceiling.

---

## Per-Call Budget

The retry counter resets for each `doHTTPWithRetry` invocation. A session with ten tool calls — each triggering an LLM completion — has ten independent 10-attempt budgets. A rate-limited call does not penalize subsequent calls in the same session.

---

## Implementation Reference

- **File:** [`goclaw/internal/llm/retry.go`](../../goclaw/internal/llm/retry.go)
- **Function:** `doHTTPWithRetry(ctx context.Context, client *http.Client, build func() (*http.Request, error)) (*http.Response, error)`
- The `build` function is called fresh on each attempt so the HTTP request body can be re-read (bodies are one-shot streams).
- Both `AnthropicClient` (`anthropic.go`) and `OllamaClient` (`ollama.go`) use this function for their streaming POST calls.
- Decision rationale: see D22 in [`goclaw/CLAUDE.md`](../../goclaw/CLAUDE.md).

---

## Post-MVP

> **Post-MVP (v2+):** Jitter on the backoff (to reduce thundering herds in multi-instance setups). Configurable attempt limits and delay ceiling via environment variables (D22). Separate retry policy per provider — Ollama and Anthropic have different transient failure patterns. Time-bounded retry for daemon/CI mode instead of count-bounded.

---

## Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created: conceptual flow, HTTP codes, unattended mode, Go implementation notes, D22. |
| 2026-04-08 | Added goclaw implementation details (`retry.go`, §2.1). |
| 2026-04-08 | Translated to English; restructured around `retry.go` facts; reference-product analysis removed. |
