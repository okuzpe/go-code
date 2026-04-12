# Open-weight 7B/8B stack (Ollama)

This document describes the **optional local model stack** for goclaw: a small general model (BASE), a coding model (CODING), a fast coordinator (FAST), and optional compaction and explore profiles. It complements [usage.md](usage.md) and [agent-profiles.md](../reference/agent-profiles.md).

## Verified tags (example host)

Run `ollama list` after pulling models. Example names that match a typical setup:

| Role | Example `ollama pull` | Notes |
| --- | --- | --- |
| BASE | `llama3:latest` | General chat; Meta Llama 3 8B class |
| CODING | `qwen2.5-coder:7b` | Code edits and tools |
| FAST coordinator | `mistral:latest` | Delegate via `spawn_agent` only; pull before using profile `stack-coordinator` |
| Compaction | `qwen2.5-coder:7b` | Smaller model for `compaction_model` when `llm_compaction` is true |
| Optional reasoning | `deepseek-r1` variants | Heavy; needs a strong GPU |

Aliases like `llama3.3` depend on the [Ollama library](https://ollama.com/library); use `ollama pull` / `ollama list` as the source of truth.

## Project template (`goclaw/.goclaw/`)

This repository includes:

- **`settings.json`** — `ollama_model` (BASE), `compaction_model` (lighter model for summaries), `llm_compaction`, `model_context_tokens`.
- **`agents/*.md`** — Custom profiles: `stack-base`, `stack-coder`, `stack-coordinator`, `stack-explore` (see frontmatter `model:`).

Switch profile in the REPL: `/profile stack-coder` (etc.). Copy these files to another project’s `.goclaw/` to reuse the stack.

## Hub profile (`stack-coordinator`) vs direct coding

- **`stack-coder`**, **`stack-base`**, and built-in **`general-purpose`** run the usual single-session agent loop: the model can call read/write/bash tools in your workspace.
- **`stack-coordinator`** is a **hub** profile (like built-in `coordinator`): it only has `spawn_agent`, `stop_task`, and `todo_write`. It does **not** edit files in the main session; it must **delegate** self-contained sub-tasks to workers. For implementation work, spawn workers with **`general-purpose`** or **`stack-coder`** so they get full coding tools. The `spawn_agent` tool schema lists every **allowed worker profile name** for your install (built-ins plus custom `*.md` agents), excluding hub-only profiles.

## Settings keys

| Key | Purpose |
| --- | --- |
| `ollama_model` | Primary model for the active profile when the profile has no `model:` override |
| `compaction_model` | Model id for LLM-driven compaction only (requires `llm_compaction: true`) |
| `model_context_tokens` | Context budget for compaction heuristics (default 32000 for Ollama if unset) |
| `llm_compaction` | When true, phase-2 compaction calls the LLM to summarize old turns |

Environment: `GOCLAW_COMPACTION_MODEL` overrides the default empty compaction model (same as `compaction_model` in JSON after merge).

## Tool calling and text-only fallback

For **autonomous agent** flows (read_file, bash, MCP, etc.), use a **tools-capable** model (for example `qwen2.5-coder:7b` or another entry from the table above) and keep **`ollama_num_ctx`** high enough for your tool schemas. If Ollama rejects tools on the wire, goclaw retries **without** function tools and the session runs in **text-only** mode for that HTTP path (`goclaw doctor` surfaces this). The runtime injects a short system note so the model stays honest about the wire while still helping with **manual** code edits (fenced blocks, steps); switching to a compatible model restores full tool use.

## Multiple models in memory (Ollama)

Ollama can keep more than one model resident if RAM/VRAM allows. Many installs use **`OLLAMA_MAX_LOADED_MODELS`** (often default `1`); increasing it reduces reload latency when switching profiles that use different `model:` values. If memory is tight, Ollama unloads a model to load another—expect a slower first request after a profile switch.

## Multimodal models

Vision-first models (e.g. Llama 4 family on some hosts) are not wired into goclaw’s text+tools REPL by model name alone; image input would require product/API changes.

## See also

- [CLAUDE.md](../../goclaw/CLAUDE.md) — environment variables and `settings.json` merge order
- [coordinator.md](coordinator.md) — `spawn_agent` and workers
