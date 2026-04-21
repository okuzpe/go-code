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
    "default": "qwen2.5-coder:7b",
    "code": "qwen2.5-coder:7b",
    "reasoning": "qwen3:8b",
    "explore": "qwen3:4b",
    "fast": "qwen2.5-coder:3b",
    "creative": "qwen3:8b"
  }
}
```

This layout matches the built-in defaults in `internal/config/config.go` (`defaultTaskModels`); merge overrides in `settings.json` per role as needed.

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

Profile **bias** when heuristics are weak: `plan` → reasoning, `explore` → explore, `verification` → fast, **`code-review` → reasoning** (including for the large `/review` user message).

## Review-heavy sessions and `code-review` profile

When using [`/review`](./code-review-workflow.md), the injected message is a full **unified diff** — often long and better served by a **reasoning** or larger coding model than by a tiny “fast” model.

**Suggested `task_models` layout** (same canonical map as above; `/review` and **`code-review`** route to **`reasoning`**, so ensure `task_models.reasoning` is strong enough for long diffs):

```json
{
  "task_model_router": "rules",
  "task_models": {
    "default": "qwen2.5-coder:7b",
    "code": "qwen2.5-coder:7b",
    "reasoning": "qwen3:8b",
    "explore": "qwen3:4b",
    "fast": "qwen2.5-coder:3b",
    "creative": "qwen3:8b"
  }
}
```

With **`llm_compaction`** and **`compaction_model`** set, long review threads can summarize older turns without paying full price on the main model every time. See [`internal/orchestrator/compaction.go`](../../goclaw/internal/orchestrator/compaction.go) and [CLAUDE.md](../../goclaw/CLAUDE.md) (compaction / `compaction_model`).

**Note:** The `rules` router maps the **`code-review`** profile to role **`reasoning`** for every user turn while that profile is active (see `classifyTaskRoleRules` in `task_model.go`), so a configured `task_models.reasoning` entry applies consistently to follow-up questions in the same review session.

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
