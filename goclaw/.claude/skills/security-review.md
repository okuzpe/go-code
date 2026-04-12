---
name: security-review
description: Use when reviewing code for security, before merging a tool, or when the user asks for a security check.
---

## Security review checklist — goclaw

### For each new tool

#### read_file
- [ ] Path canonicalized before comparing to workspace root
- [ ] Symlinks resolved and checked against workspace boundary
- [ ] Max read size enforced (512 KiB or 400 lines)
- [ ] Errors do not leak raw host paths unnecessarily to the model

#### bash
- [ ] Command validated against allowlist (not denylist)
- [ ] Allowlist includes only safe prefixes (`go`, `git status`, `ls`, `cat`, etc.)
- [ ] Timeout (30s default)
- [ ] Output truncated at 256 KiB
- [ ] Optional `cwd` constrained to workspace when provided
- [ ] Sensitive env inheritance reviewed (minimal surface)

#### web_fetch
- [ ] URL parsed with `url.Parse` before connect
- [ ] Resolved IPs checked against RFC1918, loopback, metadata IP, IPv6 link-local
- [ ] Each redirect re-validates destination (max 5 hops)
- [ ] Only `text/*` and `application/json` accepted
- [ ] Output capped at 1 MiB
- [ ] 30s timeout
- [ ] No inherited browser cookies / credentials

#### web_search
- [ ] Query bounded / sanitized where applicable
- [ ] Output limited (top 8, snippet size cap)
- [ ] Clear behavior when the provider returns empty or errors (including fallback search URL)

#### glob
- [ ] Walk stays under workspace root; pattern rejects `..`
- [ ] Match count capped (`MaxGlobMatches`)

#### grep
- [ ] Paths resolved under workspace (same boundary idea as read_file)
- [ ] Match and per-file byte caps enforced; binary files skipped

### Permission model
- [ ] Global default is `ModeAsk` (fail-closed)
- [ ] `bypassPermissions` intentionally not implemented (see CLAUDE.md § What NOT to do)
- [ ] `ModeDeny` enforced before tool execution
- [ ] `PreToolUse` hook errors block execution (orchestrator surfaces as tool error per policy)

### Hooks
- [ ] `PreToolUse` handlers avoid irreversible side effects when another handler may still block
- [ ] Project hooks under `.goclaw/` require explicit trust policy if you add auto-load later

### Session / LLM messages
- [ ] Model output is not blindly re-injected as privileged instructions
- [ ] Tool results use structured `tool_results` (Anthropic) / `tool_name` (Ollama), not ad-hoc `[tool_result]` text only
- [ ] Tool result content is data for the model, not instructions evaluated by the host

### Red flags — stop and escalate
- `exec.Command` without prior validation
- `http.Get` without SSRF checks
- File writes without path boundary checks
- `os.ReadFile` with uncleaned model paths
- Global default `ModeAllow`
