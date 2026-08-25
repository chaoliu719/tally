# tally plugin

The practice layer: skills that turn the raw [`tally-mcp`](../mcp/README.md) tools into a
bookkeeping workflow inside a conversation — precedent lookup, a conversational preview before any
write, `comment` discipline, spending analysis, and category split/merge hygiene. See the
[skills](skills/) for what each one covers.

## Prerequisites

A running `tally-mcp` instance reachable over HTTP, and its bearer token — see
[`mcp/README.md`](../mcp/README.md) for how to deploy one. You'll need two values before
installing the plugin anywhere:

- `TALLY_MCP_URL` — the server's `/mcp` endpoint, e.g. `http://localhost:16355/mcp`.
- `TALLY_MCP_TOKEN` — the same value as the server's `TALLY_MCP_TOKEN`.

Both are read by [`plugin/.mcp.json`](.mcp.json), which every install path below relies on to wire
up the `tally` MCP server.

## Install in Claude Code

```
/plugin marketplace add chaoliu719/tally
/plugin install tally@tally
```

Claude Code prompts for `TALLY_MCP_URL`/`TALLY_MCP_TOKEN` when it loads `.mcp.json`'s `${VAR}`
placeholders — enter the two values from above.

## Install in OpenClaw

OpenClaw recognizes this repo as a Claude-compatible plugin marketplace directly — no clone or
repo restructuring needed:

```
openclaw plugins install tally --marketplace chaoliu719/tally
```

Verified against a real OpenClaw 2026.7.1 instance: `plugins install` correctly resolves and
installs the plugin from this repo's marketplace, and after the two manual steps below,
`openclaw skills info record --agent <id>` reports the skill as `✓ Ready` and visible to the
model. A full live-conversation round trip (agent actually calling the `tally` MCP tools) is the
one piece still pending verification.

### The `tally` MCP server needs to be added by hand

`plugin/.mcp.json` uses Claude Code's schema. OpenClaw's bundle importer only supports stdio MCP
servers today — `plugins inspect tally --json` reports a diagnostic (`bundle MCP servers use
unsupported transports or incomplete configs (stdio only today): tally`) confirming the `tally`
server, which is HTTP, is **not** wired up automatically. Add it by hand to OpenClaw's native
`mcp.servers` config, translating the field names:

| `plugin/.mcp.json` field | OpenClaw `mcp.servers.tally` field | Value |
| --- | --- | --- |
| `type: "http"` | `transport: "streamable-http"` | (renamed, same meaning) |
| `url` | `url` | unchanged — your `TALLY_MCP_URL` |
| `headers` | `headers` | unchanged — `Authorization: Bearer <TALLY_MCP_TOKEN>` |

For example, via `openclaw config patch --stdin`:

```json
{ "mcp": { "servers": { "tally": {
  "enabled": true,
  "transport": "streamable-http",
  "url": "<TALLY_MCP_URL>",
  "headers": { "Authorization": "Bearer <TALLY_MCP_TOKEN>" }
} } } }
```

### The three skills need to be added to your agent's allowlist

An agent's `skills` list is an explicit allowlist — a freshly installed plugin's skills aren't on
it, so `openclaw skills info record --agent <id>` reports `Excluded by agent allowlist` even
though the plugin itself installed fine. Add `record`, `analysis`, and `optimize` to the target
agent's `skills` array (`agents.list[].skills` in `openclaw.json`, e.g. via `openclaw config
patch --stdin` with the agent's full `skills` array including the three new entries — patching an
array replaces it wholesale, so include the existing entries too).

## Verifying the connection

Once connected (either host), ask the assistant to record or search a transaction — this
round-trips client → the `tally` MCP server → `tally-mcp`'s query layer → SQLite. See
[`mcp/README.md`](../mcp/README.md#verifying-the-connection) for a from-scratch check of the
server itself.
