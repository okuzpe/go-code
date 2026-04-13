# Verification recipe (post-edit loop)

Professional agent workflows repeat the **same** checks after edits. goclaw does not run a hidden verifier; you define what “green” means for the repo and teach the agent to call it via **`bash`** (single allowlisted command) or **`script`** (pipes and chaining, when `allow_script` is enabled).

## Recommended: `.goclaw/verify.sh`

Add an executable script at **`<workspace>/.goclaw/verify.sh`** that runs your canonical checks (same commands you would run before opening a PR). Example template (copy and adjust):

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
go build ./...
# Add: go test ./..., golangci-lint, npm test, etc.
```

**Why a script:** the base system prompt already steers the model toward **verify** after edits. A single entry point avoids “the agent ran a different `go test` path than CI.”

**Permissions:** Default `tool_permissions` is **ask** for `bash` / `script`. For unattended automation, set explicit modes in `.goclaw/settings.json` (only after you trust the workspace).

**Security:** `script` is high risk (full shell). Prefer **`bash`** with one allowlisted command per step when possible; use **`script`** only when you need pipes (`|`, `&&`, redirects). See [tool-contract.md](../reference/tool-contract.md) and [security.md](./security.md).

## Coordinator / workers

For heavy repos, spawn a **`verification`** worker from **`coordinator`** with a task description that points at `.goclaw/verify.sh` (or your documented command). See [coordinator.md](./coordinator.md).

## Related

- [code-review-workflow.md](./code-review-workflow.md) — read-only `/review` flow  
- [model-routing.md](./model-routing.md) — faster vs stronger models per turn  
