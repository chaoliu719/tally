# tally-mcp

A pure [MCP](https://modelcontextprotocol.io) server for personal bookkeeping. It reuses
[ezbookkeeping](https://github.com/mayswind/ezbookkeeping)'s service layer (accounts, categories,
transactions, SQLite storage) as a Go library, and exposes it over MCP tools instead of a REST API
or web UI. There is no browser client, no traditional login flow, and no multi-user support — this
is a single-user server meant to sit behind a static bearer token, so an agent like Claude can record
and query a ledger directly.

## Tools

| Tool | Description |
| --- | --- |
| `list_accounts` | List every account (name, type, currency, balance). |
| `manage_account` | Create a new account. |
| `list_categories` | List every transaction category (name, type, parent id). |
| `manage_category` | Create a new category — top-level (`parent_id` omitted/`"0"`) or second-level (`parent_id` set to an existing top-level category). |
| `create_transaction` | Record one income or expense transaction against a second-level category. Updates the account balance. |
| `get_transaction` | Fetch one transaction by id. |
| `search_transactions` | List transactions, optionally filtered by time range, account, and/or category. |

Categories are two levels deep, matching ezbookkeeping: a **top-level** category is only for
grouping and cannot be used in `create_transaction`; a **second-level** category (created by
passing `parent_id`) is what transactions actually reference. All ids on the wire (account,
category, transaction) are decimal **strings**, not JSON numbers — ezbookkeeping's ids can exceed
what a JSON number safely round-trips through many MCP clients.

Not implemented in this version (left for a future change): updating/deleting accounts or
categories, tags and tag groups, custom exchange rates, batch operations, transfer-type
transactions, and any analytics/aggregation tools.

## Configuration

All configuration is via environment variables — there's no ezbookkeeping-style ini file.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `TALLY_MCP_TOKEN` | Yes | — | Static bearer token clients must send as `Authorization: Bearer <token>`. The process refuses to start without it. |
| `TALLY_DEFAULT_CURRENCY` | No | `CNY` | ISO 4217 currency code used when bootstrapping the single user. |
| `TALLY_DB_PATH` | No | `./tally.db` | Path to the SQLite file. Created (with full ezbookkeeping schema) on first run if it doesn't exist. |
| `TALLY_LISTEN_ADDR` | No | `:8080` | Address the HTTP server listens on. |

On first startup the server creates the SQLite file, runs ezbookkeeping's schema migrations, and
provisions a single internal user (with a random password that is never used or validated —
authentication is entirely via the bearer token, not ezbookkeeping's own login system). Subsequent
restarts reuse the same user.

## Build & run

Requires Go 1.26+ (ezbookkeeping's own requirement) — this project targets Go 1.27.

```bash
go build -o tally-mcp ./cmd/tally-mcp

TALLY_MCP_TOKEN=change-me \
TALLY_DB_PATH=./tally.db \
./tally-mcp
```

The server exposes:

- `POST /mcp` — the MCP endpoint (JSON-RPC over streamable HTTP), requires the bearer token.
- `GET /healthz` — unauthenticated health check, returns `200 OK`.

## Connecting a client

### Claude Code

```bash
claude mcp add --transport http tally http://localhost:8080/mcp \
  --header "Authorization: Bearer change-me"
```

Replace `change-me` with the same value as `TALLY_MCP_TOKEN`, and the host/port with wherever the
server is actually reachable (`localhost` only works if Claude Code runs on the same machine as
the server).

### Claude Desktop / claude.ai

Header-based auth for custom remote MCP connectors is a Claude.ai/Desktop feature: open
**Settings → Connectors → Add custom connector**, enter the server URL
(`http://<host>:<port>/mcp`), and add a request header named `Authorization` with the value
`Bearer change-me` (the `Bearer ` prefix must be typed in — Claude sends the header value exactly
as entered).

### Verifying the connection

Once connected, ask the assistant to list accounts (`list_accounts`) — on a fresh database this
should return an empty list, confirming the round trip: client → bearer token auth → MCP
JSON-RPC → ezbookkeeping service layer → SQLite.
