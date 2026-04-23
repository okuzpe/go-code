---
name: small-model-compat
description: Use when writing or reviewing any orchestrator, prompt injection, plan, or tool code that must work well with small local models (qwen2.5-coder:7b or similar 7B models).
---

> **Language:** English only. Rule: `.cursor/rules/agent-artifacts-english.mdc`.

## Small local model compatibility rules

goclaw's default target is **qwen2.5-coder:7b** via Ollama. 7B models have limited context (typically 8K–32K tokens effective), follow instructions less reliably than large models, and struggle with excessively long system prompts. Apply these rules whenever writing code that affects prompt content, iteration count, or tool selection.

---

### 1. Context budget — hard limits

| Injection | Limit | Configurable |
|-----------|-------|--------------|
| Skills snippet | 8 000 runes (`EffectiveSkillsMaxRunes`) | `skills_max_runes` in settings |
| Memory entries | 4 per store (`EffectiveMaxMemorySnippetEntries`) | `max_memory_snippet_entries` |
| Each memory block | 4 096 bytes (`memoryMaxBytes`) | — |
| Plan body in multi-step messages | 30 lines (`planBodyReferenceMaxLines`) | — |
| MCP tools in prompt | 0 by default (hidden, revealed via tool_search) | — |

**Rule:** every new injection needs a cap. No raw file content, no full plan bodies, no unlimited lists.

---

### 2. Prompt structure — keep it front-loaded

Small models lose track of instructions buried deep in a long system prompt. Place the most important rules at the **top** of any system prompt section:
- Static system prompt → profile overrides → project context → memory
- Memory and todo come last; they are the most variable and should not push critical instructions off the visible window.

---

### 3. Tool list — keep it short

A tool list of 20+ items confuses 7B models (they hallucinate tool names or pick the wrong one).

- MCP tools are hidden by default. Only visible tools appear in the tool spec list sent to the model.
- If a profile needs many tools, use the `tool_allowlist` to restrict to the relevant subset.
- `tool_search` lets the model self-discover tools on demand; prefer this over injecting everything upfront.

---

### 4. Iteration and tool call budgets

- Default: 32 iterations, 64 tool calls per user turn.
- For `explore` and `fast` task roles, `adaptIterBudget` halves the cap (min 4) — 7B models tend to loop on simple tasks.
- The `tui_chat_max_iterations` config (default 10) caps interactive turns separately to prevent runaway tool usage in chat mode.
- Do not raise these defaults for small models; lower them per profile if needed.

---

### 5. Plan execution

- Use `StepExecutionUserMessages` in `internal/planfile/step_execution.go` for multi-step plans.
- It caps the re-injected plan body to 30 lines; each step is delivered as a separate user message so the model processes one task at a time.
- Never inject the full plan body on every step — it pushes the current step out of focus.

---

### 6. Qwen-specific instructions

When the model is in the Qwen family (`qwenFamily(model)` in `request.go`), a suffix is appended to the system prompt:
```
[MODEL NOTE: Follow explicit numbered steps. For multi-step tasks: (1) state what you will do,
(2) group tool calls logically, (3) confirm result before the next step.
Use <thinking>...</thinking> only for complex trade-off reasoning, not for simple lookups.]
```

This is already wired. Do not add additional Qwen-specific hacks in tool execution or plan code — keep them in `request.go`.

---

### 7. Memory relevance threshold

`memoryRelevanceMinScore = 0.15` — entries below this score are not injected. This prevents flooding the prompt with tangentially related memories. Do not lower this threshold for small models; raise it if memory injection still hurts quality.

---

### 8. Checklist before merging prompt-touching code

- [ ] New injection has an explicit size cap
- [ ] Cap is configurable via `EffectiveXxx()` + settings key (for user-facing limits)
- [ ] Truncation appends a note (not silent cut)
- [ ] System prompt section is short enough to stay in the top ~4K tokens
- [ ] No full file contents injected without line/byte limit
- [ ] MCP tools registered via `reg.AddHidden` (not `reg.Register`)
