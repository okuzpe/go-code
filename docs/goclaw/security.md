# Security notes for goclaw

## Model output

Large language models can be wrong, omit details, or suggest unsafe commands. **Review** assistant output before running shell commands or applying edits, especially in production or on shared systems.

## Prompt injection

Untrusted repository content, dependencies, or pasted text can try to manipulate the agent (prompt injection). Use goclaw only on **codebases and inputs you trust**, or isolate the workspace and review tool approvals carefully.

## Workspace and hooks

Tools operate on the **current working directory** subject to permissions and your `tool_permissions` settings. Enabling **`trusted_workspace`** allows loading project `.goclaw/hooks.json` and plugin hook files — treat those as **executable configuration** with supply-chain risk.

## Secrets

Prefer **`~/.goclaw/settings.local.json`** (not committed) for API keys. Do not paste secrets into chat logs or commit them to version control.

## Skipping onboarding

`GOCLAW_NO_ONBOARDING=1` skips the first-run wizard. That does not remove the need for safe usage practices above.
