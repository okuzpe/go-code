---
name: context-budget
description: Use when adding system prompt injection, memory blocks, or any text appended to the LLM context — rules for caps, priority, and truncation.
---

> **Language:** English only. Rule: `.cursor/rules/agent-artifacts-english.mdc`.

## System prompt context budget rules

goclaw targets small local models (qwen2.5-coder:7b, ~8K–32K context). Every byte injected into the system prompt competes with tool results and conversation history. Follow these rules to avoid silent context bloat.

---

### Mandatory caps per injection type

| Injection | Current cap | Where enforced |
|-----------|-------------|----------------|
| Skills (SKILL.md) | `cfg.EffectiveSkillsMaxRunes()` default 8 000 | `internal/app/chat_wiring.go` via `skills.Collect` |
| Each memory block | `memoryMaxBytes` = 4 096 bytes | `orchestrator/request.go` `truncateMemoryBlock` |
| Memory entries fetched | `cfg.EffectiveMaxMemorySnippetEntries()` default 4 | `request.go` `RelevantContext` call |
| Plan body in multi-step | `planBodyReferenceMaxLines` = 30 lines | `planfile/step_execution.go` |
| CLAUDE.md lines | `project_context_claude_md_lines` default 60, max 200 | `projectcontext/context.go` |
| Standing orders | `project_context_standing_orders_max_lines` default 40, max 120 | `projectcontext/context.go` |
| MCP tool descriptions | `maxToolDescriptionRunes` = 220 runes | `mcp/adapter.go` `compactToolDescription` |
| MCP schema descriptions | `maxSchemaDescriptionRunes` = 160 runes | `mcp/adapter.go` `compactSchemaDescription` |

---

### Rules when adding a new injection

1. **Always add a cap.** No injection goes in without a size limit. Use rune boundaries (not byte slices) for multibyte text (`text.TruncateRunes` or `strings.SplitAfter`).

2. **Use `EffectiveXxx()` for the limit.** Hard constants are for absolute ceilings only; user-facing limits go through `config.go` with a `defaultXxx` constant and `EffectiveXxx()` method so they can be tuned in `settings.json`.

3. **Truncation note.** When content is cut, append a note so the model knows it's incomplete:
   ```go
   // Example:
   "\n…(truncated after N lines — full content at <path>)"
   ```

4. **Priority order when total is too large** (high to low):
   - Static system prompt + profile overrides (never truncated)
   - Project context (CLAUDE.md, standing orders, workspace layout)
   - Skills snippet
   - User memory (relevant entries)
   - Project memory (relevant entries)
   - Todo list
   - Budget / language hints (appended last, small)

5. **Multi-step plans:** inject the full plan body only for single-step runs. For multi-step, truncate the reference body to `planBodyReferenceMaxLines` — the model already has the current step inline.

6. **MCP tools:** hidden by default, revealed per-turn via `tool_search`. Never inject all MCP specs into the prompt. See `internal/orchestrator/request.go` `effectiveToolSpecs`.

---

### Truncation helpers

```go
// Rune-safe truncation (preserves multibyte characters):
text.TruncateRunes(block, maxRunes)

// Line-based truncation (for plan bodies, standing orders):
// see planfile/step_execution.go truncateLines()

// For memory blocks specifically:
truncateMemoryBlock(block, memoryMaxBytes)  // in request.go
```

---

### Anti-patterns to avoid

```go
// WRONG — injects full content with no cap
sys = sys + "\n\n## Full file\n\n" + entireFileContent

// WRONG — concatenates multiple blocks without total cap
sys = sys + block1 + block2 + block3

// CORRECT — cap each block, use EffectiveXxx for the limit
block = truncateMemoryBlock(block, memoryMaxBytes)
sys = sys + "\n\n## Memory\n" + block
```
