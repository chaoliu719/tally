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

This has been verified to correctly resolve the `tally` plugin (`openclaw plugins marketplace list
chaoliu719/tally --json` reports it at `./plugin`) against a real OpenClaw 2026.7.1 instance.
Whether `plugins install` itself, and the skill/MCP wiring below, come up clean end to end has not
yet been verified — treat the rest of this section as the best current understanding, not a
confirmed result.

### If the `tally` MCP server isn't picked up automatically

`plugin/.mcp.json` uses Claude Code's schema. OpenClaw's native `mcp.servers` config uses different
field names for the same thing. If installing the plugin doesn't wire up a working `tally` entry
under `mcp.servers`, add one by hand using this mapping:

| `plugin/.mcp.json` field | OpenClaw `mcp.servers.tally` field | Value |
| --- | --- | --- |
| `type: "http"` | `transport: "streamable-http"` | (renamed, same meaning) |
| `url` | `url` | unchanged — your `TALLY_MCP_URL` |
| `headers` | `headers` | unchanged — `Authorization: Bearer <TALLY_MCP_TOKEN>` |

### Enabling the skills

A freshly installed plugin's skills may land disabled by default. If `record`/`analysis`/
`optimize` don't show up for your agent after installing:

1. Enable each one under `skills.entries` in OpenClaw's config (`enabled: true`).
2. Add `record`, `analysis`, and `optimize` to the target agent's `skills` list.

## Verifying the connection

Once connected (either host), ask the assistant to record or search a transaction — this
round-trips client → the `tally` MCP server → `tally-mcp`'s query layer → SQLite. See
[`mcp/README.md`](../mcp/README.md#verifying-the-connection) for a from-scratch check of the
server itself.
