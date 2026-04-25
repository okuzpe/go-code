# Code review workflow (`/review` + `code-review` profile)

This document describes the **shipped** flow for a **read-only**, git-anchored review turn: the CLI runs `git diff` in the workspace, injects stdout into one user message, switches to the **`code-review`** profile, and streams the model response. There is **no** GitHub/GitLab API integration; for PR parity on a host, see [ide-pr-parity.md](./ide-pr-parity.md).

## Slash command: `/review`

| Input | Git command used |
|--------|------------------|
| `/review` | `git diff HEAD` (all changes vs `HEAD`: working tree + index) |
| `/review --staged` or `/review --cached` | `git diff --cached` (staged only) |
| `/review main HEAD` | `git diff main HEAD` (two revs; tokens validated) |
| `/review main...HEAD` | `git diff main...HEAD` (merge-base diff; single rev token with `...`) |
| `/review HEAD -- internal/foo.go` | `git diff HEAD -- internal/foo.go` (rev + pathspec) |

**Limits:** Up to **four** tokens after `/review` for the custom forms above. Tokens must match safe characters for revisions and paths (letters, digits, `._~^@/:-*?\` and merge-base syntax). Unsupported `git diff` flags are rejected so arbitrary shell is never passed through this path.

**Truncation:** Diff text is capped at **120 KiB**; if truncated, narrow the diff with a pathspec or a smaller rev range.

## Profile: `code-review`

Defined in [`goclaw/internal/agents/profile.go`](../../goclaw/internal/agents/profile.go) as `agents.CodeReview`.

- **Workspace writes:** `write_file`, `edit_file`, and `patch` are **not** on the tool allowlist (same effect as a read-only review for the repo).  
- **Shell:** `bash` remains available for **non-destructive** checks (e.g. `git log`, `go vet` on one package), subject to the normal bash allowlist and permission modes.  
- **Reads:** `read_file`, `glob`, `grep`, `web_fetch`, `web_search`, `todo_write`.

Use **`/mode build`** (or `/audit`) when the user wants edits, not only comments.

## Skill template

Optional checklist copy lives in [`.claude/skills/code-review.md`](../../.claude/skills/code-review.md). If that file is present under a [skills search root](../../goclaw/internal/skills/), its body is injected into the system prompt (bounded size).

## Verification after fixes

Reviews from `/review` do not run your test suite. After you apply changes in another profile, use a stable project script — see [verification-recipe.md](./verification-recipe.md).

## Model routing

For long diffs and multi-turn review chat, prefer **`task_models`** with a **`reasoning`** (or larger) model for the `code-review` profile; see [model-routing.md](./model-routing.md#review-heavy-sessions-and-code-review-profile).
