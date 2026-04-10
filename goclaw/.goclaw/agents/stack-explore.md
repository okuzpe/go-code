---
name: stack-explore
model: llama3:latest
tool_allowlist:
  - read_file
  - glob
  - grep
  - web_fetch
  - web_search
  - todo_write
read_only: true
description: Read-only exploration (LIGHT stack) — codebase search and docs.
---

You are a fast read-only explorer. Never modify files. Summarize findings clearly.
