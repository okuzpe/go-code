# Per-turn task model routing

goclaw can pick a **different model id per user turn** from a small table of **roles** (code, reasoning, fast, …) using either **heuristics** or an optional **LLM classifier** call. This is **not** automatic “best model in the world”—you assign provider-specific model ids in settings.

## When routing runs

Routing applies only if **all** of the following hold:

1. `task_model_router` is `rules` or `llm` (not `off`).
2. `task_models` is a non-empty object in merged settings.
3. The active agent profile does **not** set `model` (no `ModelOverride` from built-in profile or custom `.goclaw/agents/*.md` frontmatter).

**Precedence:** profile `model` → routed model from `task_models` → global default from `Model()` (provider-specific).

## Settings

In `~/.goclaw/settings.json` and/or `<project>/.goclaw/settings.json` (merged in the usual order):

```json
{
  "task_model_router": "rules",
  "task_models": {
    "default": "qwen2.5-coder:14b",
    "code": "qwen2.5-coder:14b",
    "reasoning": "llama3.1:8b-instruct-q4_K_M",
    "fast": "phi3:mini",
    "explore": "qwen2.5-coder:7b",
    "creative": "llama3.1:8b-instruct-q4_K_M"
  }
}
```

Optional keys:

- **`task_model_router`**: `off` (default), `rules`, or `llm`. Shorthand `on` / `true` / `yes` is normalized to `rules`. Env: **`GOCLAW_TASK_MODEL_ROUTER`**.
- **`task_model_router_model`**: model id used **only** for the short JSON classification when `task_model_router` is `llm`. If omitted, **`ModelForCompaction()`** is used (so a dedicated `compaction_model` can double as the router model). Env: **`GOCLAW_TASK_MODEL_ROUTER_MODEL`**.

## CLI

```bash
goclaw --task-model-router=rules
goclaw --task-model-router=llm
```

Non-empty flag overrides the merged setting for that process.

## Roles

| Role | Intended use |
|------|----------------|
| `default` | Fallback when no other role matches; also used if a role key is missing from `task_models`. |
| `code` | Coding, debugging, tests, stack traces, fenced code blocks, refactor keywords. |
| `reasoning` | Tradeoffs, analysis, step-by-step “why”. |
| `fast` | Short, single-line prompts when the profile fallback is still generic `default`. |
| `explore` | Locate code / repo navigation style prompts. |
| `creative` | Brainstorming, marketing copy, prose. |

Profile **bias** when heuristics are weak: `plan` → reasoning, `explore` → explore, `verification` → fast.

## `rules` vs `llm`

- **`rules`**: No extra API call. Fast, predictable, may misclassify edge cases.
- **`llm`**: One short completion with a constrained JSON `{"role":"…"}` answer, then the same `task_models` lookup. On failure or empty output, falls back to `rules`. Adds latency and token cost (`task_model_router_model` / compaction model).

## Coordinator and workers

The **main** session uses routing as above. **`spawn_agent` workers** use their **worker profile**; custom agents can set **`model:`** in YAML frontmatter to pin a worker model regardless of routing.

## Debugging

At **debug** log level (`GOCLAW_LOG=debug`), each turn logs `task model routing` with `role`, effective `model`, and `reason`. **`goclaw doctor`** shows `task_model_router` and how many roles are configured.

## Implementation map

- Config / merge: `internal/config/config.go`, `loader.go`
- Classification + wiring: `internal/orchestrator/task_model.go`, `task_model_llm.go`, `request.go`, `orchestrator.go`
