# Editor integration — golden path (goclaw + local MCP)

This page is the **shortest reproducible path** to connect a **local editor MCP server** to goclaw. Deep design notes stay in [ide-bridge.md](../reference/ide-bridge.md); PR vs IDE scope is in [ide-pr-parity.md](./ide-pr-parity.md).

## What you get

1. **Lockfile MCP** — Your editor (or a small helper) exposes an MCP endpoint on **loopback**. goclaw reads **`~/.goclaw/ide/*.json`**, picks the **first valid** file (sorted by name), and adds a synthetic MCP server **`id: "ide"`** so tools appear as `mcp__ide__<tool_name>`.
2. **Post-tool notify (optional)** — **`GOCLAW_IDE_NOTIFY_URL`** receives a small JSON POST after each tool (progress UI in the extension). Same **loopback-only** rule as [ide-bridge.md §6–§7](../reference/ide-bridge.md).

## Prerequisites

- **Ollama** running; **`goclaw doctor`** clean for your usual workspace.
- An MCP server from the editor that listens on **`127.0.0.1`**, **`localhost`**, or **`::1`** only (goclaw rejects non-loopback URLs for this discovery path unless you configure HTTP MCP manually under `mcp_servers` with `mcp_allow_remote_urls` — see [mcp-remote.md](./mcp-remote.md)).

## Step 1 — Create the lockfile directory

```bash
mkdir -p ~/.goclaw/ide
```

## Step 2 — Write a JSON lockfile

Copy [examples/ide-mcp-endpoint.example.json](./examples/ide-mcp-endpoint.example.json) and adjust:

| Field | Meaning |
|-------|---------|
| `url` | Full MCP HTTP URL your editor exposes (must be loopback). |
| `headers` | Optional; e.g. `Authorization` if the server requires a static token. |

**Naming:** any `*.json` name works. If multiple files exist, goclaw sorts names lexicographically and uses the **first** that parses and validates — use a prefix like `00-vscode.json` if you need a stable order.

**Remove** the lockfile when the editor MCP server is stopped so goclaw does not keep a stale endpoint.

## Step 3 — Enable bridge in settings

In **`~/.goclaw/settings.json`** or **`.goclaw/settings.json`** (project), set:

```json
{
  "ide_bridge_mcp": true
}
```

Merge rules: [usage.md — Configuration](./usage.md#configuration).

## Step 4 — Optional: post-tool notifications

```bash
export GOCLAW_IDE_NOTIFY_URL="http://127.0.0.1:9999/goclaw-tool"
```

Your listener must accept **POST** with JSON body `{"tool","result_bytes","is_error"}` (see [ide-bridge.md §7](../reference/ide-bridge.md)). Invalid or non-loopback URLs are **logged at warn** and ignored.

## Step 5 — Verify

```bash
cd /path/to/your/repo
goclaw doctor
```

In **`doctor`** output, open the **`ide bridge:`** section:

- Lockfile discovery **✓** when a valid `~/.goclaw/ide/*.json` exists and `ide_bridge_mcp` is true.
- **`mcp servers:`** should list **`ide`** with **✓ connected** if the editor’s server is running.

Then start **`goclaw`** as usual; approve or allow **`mcp__ide__*`** tools in `tool_permissions` if you use **ask** mode.

## Troubleshooting

| Symptom | What to check |
|---------|----------------|
| Startup **warn**: `ide bridge mcp: no MCP endpoint from lockfiles` | Directory missing, no `.json` files, bad JSON, empty `url`, or URL not loopback. Run **`goclaw doctor`** for the same `err` text. |
| **`ide`** in doctor but **✗ not connected** | MCP server not running, wrong port/path, TLS mismatch (try `http://` for local dev), or auth (401) — add `headers` in the lockfile or `mcp_servers` pattern from [mcp.md](../reference/mcp.md). |
| **warn**: `GOCLAW_IDE_NOTIFY_URL ignored` | Scheme must be `http`/`https` and host must be loopback. |
| Tools not visible to the model | Confirm **`ide`** appears under **`mcp servers:`** connected; check profile allowlists; unlisted MCP tools default to **ask** — you may need to approve the first call. |

## VS Code / Cursor–class editors (pattern)

There is **no** first-party goclaw extension in this repo yet. The intended pattern is:

1. A **workspace or UI extension** starts (or documents) an MCP HTTP server on loopback.
2. On activate, it writes **`~/.goclaw/ide/<editor>.json`** with the live `url` (and deletes it on deactivate).
3. The user sets **`ide_bridge_mcp`: true** once.

If your stack only supports **stdio** MCP, register it under **`mcp_servers`** in settings instead of the lockfile path; the lockfile path is for **HTTP** endpoints matching [internal/mcp/http.go](../../goclaw/internal/mcp/http.go).

## Changelog

| Date | Change |
|------|--------|
| 2026-04-14 | Initial golden path, example JSON, doctor cross-checks |
