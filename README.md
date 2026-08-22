# tally-mcp

A pure [MCP](https://modelcontextprotocol.io) server for personal bookkeeping, backed by its own
SQLite schema and query layer (via [sqlc](https://sqlc.dev)) — no REST API, no web UI, no
third-party bookkeeping library underneath. There is no browser client, no traditional login flow,
and no multi-user support — this is a single-user server meant to sit behind a static bearer
token, so an agent like Claude can record and query a ledger directly. The database itself is the
one implicit ledger; there's no user account to create or log into.

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

Categories are two levels deep: a **top-level** category is only for grouping and cannot be used
in `create_transaction`; a **second-level** category (created by passing `parent_id`) is what
transactions actually reference. All ids on the wire (account, category, transaction) are decimal
**strings**, not JSON numbers, so no MCP client ever risks losing precision decoding one into a
JSON number.

Amounts (`balance`, `amount`) are always in the account currency's smallest unit, but how many
decimal places that represents **varies by currency** — 2 for most currencies (e.g. USD, CNY), 0
for others (e.g. JPY, KRW), and 3 for a handful (e.g. BHD, KWD, OMR), per the real ISO 4217
standard. There is no single fixed "divide by 100" rule that works for every currency.

Not implemented in this version (left for a future change): updating/deleting accounts or
categories, tags and tag groups, custom exchange rates, batch operations, transfer-type
transactions, and any analytics/aggregation tools.

## Configuration

All configuration is via environment variables.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `TALLY_MCP_TOKEN` | Yes | — | Static bearer token clients must send as `Authorization: Bearer <token>`. The process refuses to start without it. |
| `TALLY_DB_PATH` | No | `./tally.db` | Path to the SQLite file. Created (with tally's own schema) on first run if it doesn't exist. |
| `TALLY_LISTEN_ADDR` | No | `:8080` | Address the HTTP server listens on. |

On first startup the server creates the SQLite file (if needed) and applies tally's schema
(`CREATE TABLE IF NOT EXISTS`, so it's safe to run on every startup). There is no user account to
create — the database itself is the single, implicit ledger, and authentication is entirely via
the bearer token above.

## Build & run

Requires Go 1.27+. The SQLite driver ([modernc.org/sqlite](https://modernc.org/sqlite)) is pure
Go, so no cgo or C toolchain is needed to build or cross-compile.

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
JSON-RPC → tally's query layer → SQLite.
