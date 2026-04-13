# IDE and PR parity (Wave A / Wave B)

goclaw is primarily a **local CLI** over **Ollama**. “Parity” with hosted assistants that live inside GitHub or the editor is **partial** by design; this page maps what exists today to the roadmap waves so you can choose the next investment.

## What is shipped today

| Area | Behavior | Primary references |
|------|----------|-------------------|
| **Localhost IDE ping** | After each tool, optional POST to `GOCLAW_IDE_NOTIFY_URL` (loopback). | [`goclaw/internal/ide`](../../goclaw/internal/ide), [ide-bridge.md](../reference/ide-bridge.md) |
| **MCP** | stdio servers and **streamable HTTP** MCP with loopback-by-default URL policy; tools appear as `mcp__<id>__<name>`. | [mcp.md](../reference/mcp.md), [mcp-remote.md](./mcp-remote.md) |
| **Lockfile MCP** | `ide_bridge_mcp` can surface editor-discovered MCP config. | [ide-bridge.md](../reference/ide-bridge.md) |
| **PR / host APIs** | No first-party `gh` PR comment tool or GitHub App; use **`bash`** / **`script`** within allowlist and your token setup if you accept that risk surface. | [tool-contract.md](../reference/tool-contract.md) |

## Wave A — deeper editor integration

**Goal:** smoother “open in editor” / file sync / richer MCP toward the extension.

**First steps (from [roadmap.md](./roadmap.md)):** ship or document one reference extension flow; tighten discovery and failure modes; align [ide-bridge.md](../reference/ide-bridge.md) contract sections with `internal/ide`.

## Wave B — MCP enterprise

**Goal:** OAuth, token refresh, optional WebSocket transport, stricter policy for non-loopback MCP URLs.

**First steps:** see [mcp-remote.md](./mcp-remote.md); do not enable `mcp_allow_remote_urls` without reading the SSRF posture in settings docs.

## Relation to `/review`

[`/review`](./code-review-workflow.md) gives a **local git-anchored** review turn. It does **not** post inline comments on a PR; combine it with your own `gh` automation (via **`script`** or external CI) if you need host-side artifacts.
