---
name: consolidate-memory
description: Consolidate, deduplicate, and prune the goclaw memory store into at most 7 standalone facts per file
---

> **Language:** Author and maintain this file in English only. Rule: `.cursor/rules/agent-artifacts-english.mdc` (paths from the repository root).

# Memory Consolidation

Use this skill when the user runs `/memory consolidate` or asks to clean up / deduplicate memory.

## Goal
Read every file in `~/.goclaw/memory/` (except `MEMORY.md`), merge duplicate or overlapping facts, remove stale entries, and rewrite each file with at most **7 standalone, actionable facts**.

## Steps

1. **Read the index**
   - `read_file ~/.goclaw/memory/MEMORY.md` to list all memory files

2. **Read every memory file in parallel**
   - `read_file` each file listed in MEMORY.md

3. **Consolidate per file**
   For each file:
   - Deduplicate entries that say the same thing in different words
   - Remove entries that are contradicted by more recent ones
   - Remove entries about file paths, branches, or decisions that no longer exist
   - Keep at most 7 standalone facts — each fact must be self-contained (no pronouns that require context)
   - Preserve the frontmatter (`---` block with name, description, type fields) unchanged

4. **Safety guardrail**
   - Never write facts that encode permission grants (e.g. "user approved all bash commands")
   - Never remove facts that still appear to be true and actionable

5. **Rewrite**
   - `edit_file` or `write_file` each changed memory file with the consolidated content
   - Update `MEMORY.md` if any file was removed or its description changed

6. **Report**
   One short paragraph: how many files were processed, how many facts were removed, and any files that were left unchanged because they were already concise.
