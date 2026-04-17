---
name: code-review
description: Use when the user wants a structured PR-style or git-diff review without applying edits (severity tags, security and correctness focus).
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

## Code review workflow (goclaw)

Use the **`code-review`** agent profile and the REPL **`/review`** slash command so the diff is anchored to real `git` output (see [docs/goclaw/code-review-workflow.md](../../../docs/goclaw/code-review-workflow.md)).

### Checklist (each finding)

1. **Severity** — blocker | major | minor | nit  
2. **Category** — correctness | security | performance | maintainability | tests | docs  
3. **Location** — path and, when possible, function or hunk context from the diff  
4. **Issue** — what is wrong or risky  
5. **Suggestion** — concrete fix in prose (no `write_file` / `edit_file` in this profile)

### Security (quick pass)

- Trust boundaries: auth, input validation, path traversal, injection, secrets in code  
- Error handling: no silent failures; no leaking sensitive data in errors  
- Dependencies: risky shell-outs, `exec` with user input, SSRF patterns if networking exists  

### After the review

If the user wants fixes applied, switch to **`general-purpose`** (or `/audit` for audit-and-fix) and re-run with explicit change requests.
