# Shell (Bash) security — reference and Go mapping

Depth linked to [CLAUDE.md](../../goclaw/CLAUDE.md) (bash / permissions). Third-party explainer: [Bash Security — claude-code-explain](https://claude-code-explain.helmcode.com/bash-security). Map: [docs-map.md](../docs-map.md).

---

## 1. Scope

The analyzed product has **several layers**: command validators, permissions pipeline, sandbox (e.g. seatbelt/bubblewrap on Unix), path validation, environment sanitization, and category rules (prompt injection via tool results).

**goclaw today** does not attempt parity with dozens of validators from the reference product: it does implement **minimum defense in depth** aligned with **D4** (Windows), **D5** (modes), and [yolo-classifier.md](./yolo-classifier.md) when auto mode / **D17** is active.

### 1.1 **goclaw** implementation (today)

The `bash` tool in [`goclaw/internal/tools/bash.go`](../../goclaw/internal/tools/bash.go) combines:

- **Binary and `git` subcommand allowlist** (see code).
- **Shell syntax scanning** (`rejectShellMetacharacters`): blocks pipes (`|`), separators (`;`, `&&`), redirections (`>`, `<`), subshells `(...)`, substitution `$(...)`, backticks, and unquoted `&` (URLs with `&` must be quoted). This prevents a first-token allowlist entry (e.g. `curl`) from running non-listed binaries via `| sh`.
- **Permissions** [README — Permissions](../../goclaw/README.md): `ask` mode requests confirmation on stderr before executing.

Contract detail: [tool-contract.md](./tool-contract.md) `bash` row.

### 1.2 Windows vs POSIX

On **Unix**, the tool runs `/bin/bash -c "<command>"` when validation passes.

On **Windows**, resolution order is: **`bash.exe`** on `PATH` (e.g. Git for Windows), else **`sh.exe`**, else **`cmd.exe /C`**. The binary allowlist is primarily **Unix-style** (`ls`, `grep`, …). When falling back to **CMD** without a POSIX shell, goclaw additionally allows a small built-in set: **`dir`**, **`where`**, **`type`** (CMD builtins), plus any allowlisted executable that exists on `PATH` as a normal Windows binary (e.g. **`git`**, **`go`**). If a command is rejected and no `bash`/`sh` is on `PATH`, the error text suggests installing **Git for Windows** or using **WSL** for full parity with Linux-style invocations.

---

## 2. Go mapping (reference)

| Layer | Notes |
|-------|-------|
| Parsing / tokenization | Limit metacharacters, pipeline depth, obvious denies (`rm -rf /`, etc.) |
| Permissions | Always before `exec`; see §2.3 |
| Paths | Work inside the workspace; deny escapes if applicable |
| Environment | Subprocess with reduced env (no unfiltered inherited secrets) |
| OS sandbox | Not implemented (OS-level); Windows is the hard case |
| Classifier | **D17** in goclaw: risk rules + threshold; different pattern from the LLM classifier in the reference product |

---

## 3. Changelog

| Date | Change |
|------|--------|
| 2026-04-07 | Created stub: layers, D4/D5, helmcode link; docs map |
| 2026-04-08 | §1.1 goclaw: allowlist + metacharacters, link to `bash.go` and [tool-contract.md](./tool-contract.md) |
| 2026-04-12 | Translated from Spanish to English |
| 2026-04-12 | §1.2 Windows: bash/sh/cmd fallback, CMD extras (dir/where/type), Git Bash / WSL guidance |
