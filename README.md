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
| `manage_account` | Create, update, or delete an account, via `operation=create/update/delete`. Delete is a two-step preview → apply confirmation (see below). |
| `list_categories` | List every transaction category (name, parent id). |
| `manage_category` | Create, update, or delete a transaction category, via `operation=create/update/delete`. Categories nest to any depth. Delete is a two-step preview → apply confirmation (see below). |
| `create_transaction` | Record one income, expense, or balance_adjustment transaction. income/expense reference an existing category (any category in the ledger); balance_adjustment corrects an account's balance directly with a signed amount and no category. Updates the account balance. |
| `get_transaction` | Fetch one transaction by id. |
| `search_transactions` | List transactions, optionally filtered by time range, account, and/or category, sorted oldest first. Paginated via `limit` (default 50, max 200) and `cursor`; the response includes `next_cursor` when more results remain. |
| `update_transaction` | Replace every field of an existing transaction by id (same validation rules as `create_transaction`). Full replacement, not a partial update; no confirmation required. |
| `delete_transaction` | Delete a transaction by id. Two-step preview → apply confirmation (see below); unlike account/category deletion, any existing transaction can be deleted. |

Categories can nest to any depth — `parent_id` may point at any existing category, and any
category (top-level or nested) can be referenced by `create_transaction`. All ids on the wire
(account, category, transaction) are decimal **strings**, not JSON numbers, so no MCP client ever
risks losing precision decoding one into a JSON number.

Amounts (`balance`, `amount`) are always in the account currency's smallest unit, but how many
decimal places that represents **varies by currency** — 2 for most currencies (e.g. USD, CNY), 0
for others (e.g. JPY, KRW), and 3 for a handful (e.g. BHD, KWD, OMR), per the real ISO 4217
standard. There is no single fixed "divide by 100" rule that works for every currency.

### Deleting an account, category, or transaction

`manage_account`/`manage_category` with `operation=delete`, and `delete_transaction`, are all a
two-step confirmation, not a single-call delete:

1. Call without `confirmation_token` (for `manage_account`/`manage_category`, with
   `operation=delete` and the target `id`; for `delete_transaction`, with the target `id`). If the
   resource can currently be deleted (no referencing transactions for an account; no child
   categories or referencing transactions for a category; a transaction has no such gate — any
   existing transaction can always be deleted), the response has `status=pending_confirmation` and
   includes a `confirmation_token` (and its expiry).
2. Call again the same way, this time passing that `confirmation_token`. The resource is deleted
   and the response has `status=deleted`.

A `confirmation_token` expires after 15 minutes and is invalidated if the resource's state changes
between the two calls (e.g. a transaction gets recorded against an account, or the transaction
itself is edited, in the meantime) — in either case, preview again to get a fresh token.

Deleting every transaction referencing an account or category (via `delete_transaction`) is how you
clear the reference block on `manage_account`/`manage_category`'s `operation=delete` — the account
or category itself is otherwise never automatically deleted or modified by deleting transactions.

Not implemented in this version (left for a future change): tags and tag groups, custom exchange
rates, batch operations, transfer-type transactions, and any analytics/aggregation tools.

## Configuration

All configuration is via environment variables.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `TALLY_MCP_TOKEN` | Yes | — | Static bearer token clients must send as `Authorization: Bearer <token>`. The process refuses to start without it. |
| `TALLY_CONFIRMATION_SECRET` | Yes | — | Secret used to sign/verify `confirmation_token`s for destructive operations (account/category/transaction delete). Independent of `TALLY_MCP_TOKEN`. The process refuses to start without it. |
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
TALLY_CONFIRMATION_SECRET=change-me-too \
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
