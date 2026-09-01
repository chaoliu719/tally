# tally-mcp

A pure [MCP](https://modelcontextprotocol.io) server for personal bookkeeping, backed by its own
SQLite schema and query layer (via [sqlc](https://sqlc.dev)) — no REST API, no web UI, no
third-party bookkeeping library underneath. There is no browser client, no traditional login flow,
and no multi-user support — this is a single-user server meant to sit behind a static bearer
token, so an agent like Claude can record and query a ledger directly. A single user can maintain
multiple, fully isolated **ledgers** in the same database (see below); there's still no user
account to create or log into.

## Tools

| Tool | Description |
| --- | --- |
| `list_ledgers` | List every ledger (id, name, comment). |
| `manage_ledger` | Create, update, or delete a ledger, via `operation=create/update/delete`. Delete is a two-step preview → apply confirmation (see below); a ledger must be empty (no sources, categories, or transactions) to delete. |
| `list_sources` | List every source (id, name) in one ledger — where a transaction's money comes from or goes to. |
| `manage_source` | Create, update, or delete a source within one ledger, via `operation=create/update/delete`. Delete is a two-step preview → apply confirmation (see below). |
| `list_categories` | List every transaction category (name, parent id) in one ledger. |
| `manage_category` | Create, update, or delete a transaction category within one ledger, via `operation=create/update/delete`. Categories nest to any depth. Delete is a two-step preview → apply confirmation (see below). |
| `create_transaction` | Record one income or expense transaction in one ledger, referencing an existing source and category (any category in that ledger). |
| `get_transaction` | Fetch one transaction by id (and its ledger id). |
| `search_transactions` | List transactions in one ledger, optionally filtered by time range, source, and/or category, sorted oldest first. Paginated via `limit` (default 50, max 200) and `cursor`; the response includes `next_cursor` when more results remain. |
| `update_transaction` | Replace every field of an existing transaction by id (same validation rules as `create_transaction`). Full replacement, not a partial update; no confirmation required. |
| `delete_transaction` | Delete a transaction by id. Two-step preview → apply confirmation (see below); unlike source/category deletion, any existing transaction can be deleted. |
| `get_financial_summary` | Aggregate income/expense/net totals over an optional time range within one ledger, grouped by currency, and broken down by category and by source. Read-only. |

Every source, category, and transaction belongs to exactly one ledger, and every tool above (other
than `list_ledgers`/`manage_ledger` themselves) takes a required `ledger_id` — there's no notion of
a "current" ledger held server-side, so callers pass it explicitly on every call. Ledgers are fully
isolated: nothing in one ledger (including same-named sources/categories) is visible to or
reachable from another. A brand-new database starts with zero ledgers; create one with
`manage_ledger` before creating any sources, categories, or transactions. A ledger can only be
deleted once it's empty — clear out its sources, categories, and transactions first.

Categories can nest to any depth — `parent_id` may point at any existing category in the same
ledger, and any category (top-level or nested) can be referenced by `create_transaction`. All ids
on the wire (ledger, source, category, transaction) are decimal **strings**, not JSON numbers, so
no MCP client ever risks losing precision decoding one into a JSON number.

A source has no balance and no currency of its own — it is just a label for where a transaction's
money comes from or goes to. Currency lives on the transaction: `create_transaction`/
`update_transaction` take a required `currency` field, and `amount` is always in that currency's
smallest unit, but how many decimal places that represents **varies by currency** — 2 for most
currencies (e.g. USD, CNY), 0 for others (e.g. JPY, KRW), and 3 for a handful (e.g. BHD, KWD, OMR),
per the real ISO 4217 standard. There is no single fixed "divide by 100" rule that works for every
currency.

### Deleting a ledger, source, category, or transaction

`manage_ledger`/`manage_source`/`manage_category` with `operation=delete`, and
`delete_transaction`, are all a two-step confirmation, not a single-call delete:

1. Call without `confirmation_token` (for `manage_ledger`/`manage_source`/`manage_category`, with
   `operation=delete` and the target `id`; for `delete_transaction`, with the target `id`). If the
   resource can currently be deleted (no sources, categories, or transactions for a ledger; no
   referencing transactions for a source; no child categories or referencing transactions for a
   category; a transaction has no such gate — any existing transaction can always be deleted), the
   response has `status=pending_confirmation` and includes a `confirmation_token` (and its
   expiry).
2. Call again the same way, this time passing that `confirmation_token`. The resource is deleted
   and the response has `status=deleted`.

A `confirmation_token` expires after 15 minutes and is invalidated if the resource's state changes
between the two calls (e.g. a transaction gets recorded against a source, or the transaction
itself is edited, in the meantime) — in either case, preview again to get a fresh token.

Deleting every transaction referencing a source or category (via `delete_transaction`) is how you
clear the reference block on `manage_source`/`manage_category`'s `operation=delete` — the source
or category itself is otherwise never automatically deleted or modified by deleting transactions.

Not implemented in this version (left for a future change): tags and tag groups, custom exchange
rates, batch operations, and trend/reconciliation-style analytics beyond `get_financial_summary`.

## Configuration

All configuration is via environment variables.

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `TALLY_MCP_TOKEN` | Yes | — | Static bearer token clients may send as `Authorization: Bearer <token>`. Also the login gate for the OAuth `/authorize` step. The process refuses to start without it. |
| `TALLY_CONFIRMATION_SECRET` | Yes | — | Secret used to sign/verify `confirmation_token`s for destructive operations (ledger/source/category/transaction delete). Independent of `TALLY_MCP_TOKEN`. The process refuses to start without it. |
| `TALLY_OAUTH_SIGNING_SECRET` | Yes | — | Secret that signs/verifies the OAuth authorization codes, access tokens, and client IDs. Independent of the two secrets above. Rotating it invalidates every issued access token at once. The process refuses to start without it. |
| `TALLY_PUBLIC_BASE_URL` | Yes | — | The externally reachable origin, no trailing slash (e.g. `https://tally.liuchao.life`). Anchors the OAuth issuer, the well-known metadata URLs, and the canonical resource URI (`<base>/mcp`) that access tokens are bound to. The process refuses to start without it. |
| `TALLY_DB_PATH` | No | `./tally.db` | Path to the SQLite file. Created (with tally's own schema) on first run if it doesn't exist. |
| `TALLY_LISTEN_ADDR` | No | `:16355` | Address the HTTP server listens on. |

On first startup the server creates the SQLite file (if needed) and applies tally's schema
(`CREATE TABLE IF NOT EXISTS`, so it's safe to run on every startup) with zero ledgers — none are
created automatically. There is no user account to create; authentication is entirely via the
bearer token above.

## Deploy with Docker

Prebuilt images (`linux/amd64` + `linux/arm64`) are published to
[ghcr.io/chaoliu719/tally-mcp](https://github.com/chaoliu719/tally/pkgs/container/tally-mcp)
on every version tag. No clone or Go toolchain needed on the deploy machine:

```bash
curl -fsSL https://raw.githubusercontent.com/chaoliu719/tally/main/mcp/install.sh | bash
```

This fetches `docker-compose.yml` and writes a `./tally-mcp/.env` with freshly generated
`TALLY_MCP_TOKEN`/`TALLY_CONFIRMATION_SECRET`/`TALLY_OAUTH_SIGNING_SECRET` values (re-running it
leaves an existing `.env` untouched). **Then edit `TALLY_PUBLIC_BASE_URL` in `.env`** to the URL
clients will actually use (e.g. `https://tally.example.com`), and:

```bash
cd tally-mcp
docker compose up -d
```

The SQLite file persists in a named Docker volume across restarts. See `.env.example` in the repo
for what each variable does.

## Build & run

Requires Go 1.27+. The SQLite driver ([modernc.org/sqlite](https://modernc.org/sqlite)) is pure
Go, so no cgo or C toolchain is needed to build or cross-compile.

```bash
go build -o tally-mcp ./cmd/tally-mcp

TALLY_MCP_TOKEN=change-me \
TALLY_CONFIRMATION_SECRET=change-me-too \
TALLY_OAUTH_SIGNING_SECRET=change-me-three \
TALLY_PUBLIC_BASE_URL=http://localhost:16355 \
TALLY_DB_PATH=./tally.db \
./tally-mcp
```

The server exposes:

- `POST /mcp` — the MCP endpoint (JSON-RPC over streamable HTTP). Requires either the static
  bearer token or an OAuth access token this server issued; an unauthenticated request gets a
  `401` whose `WWW-Authenticate` header points at the protected-resource metadata.
- `GET /healthz` — unauthenticated health check, returns `200 OK`.
- `GET /.well-known/oauth-protected-resource`, `GET /.well-known/oauth-authorization-server` —
  unauthenticated OAuth discovery documents (RFC 9728 / RFC 8414).
- `GET|POST /authorize`, `POST /token`, `POST /register` — the built-in OAuth 2.1 authorization
  server. `/authorize` presents a one-field form asking for the static token as the login gate;
  everything is stateless (HMAC-signed codes and tokens, no store). See
  `openspec/specs/mcp-oauth-authorization/spec.md`.

### Authentication: static token vs OAuth

The static `TALLY_MCP_TOKEN` works directly as a bearer token — the simplest path for Claude Code
and Claude Desktop. claude.ai's web custom-connector flow only speaks OAuth, so the server also
runs a minimal single-user OAuth 2.1 authorization server (authorization code + PKCE, dynamic
client registration, ~1h access tokens, no refresh token, no scopes, no per-token revocation).
The one credential in both cases is the static token.

## Connecting a client

### Claude Code

```bash
claude mcp add --transport http tally http://localhost:16355/mcp \
  --header "Authorization: Bearer change-me"
```

Replace `change-me` with the same value as `TALLY_MCP_TOKEN`, and the host/port with wherever the
server is actually reachable (`localhost` only works if Claude Code runs on the same machine as
the server).

### Claude Desktop

Open **Settings → Connectors → Add custom connector**, enter the server URL
(`https://<host>/mcp`), and add a request header named `Authorization` with the value
`Bearer change-me` (the `Bearer ` prefix must be typed in — Claude sends the header value exactly
as entered).

### claude.ai (web)

The web connector only speaks OAuth, so use the built-in authorization server: open
**Settings → Connectors → Add custom connector**, enter the server URL (`https://<host>/mcp`),
leave the OAuth fields blank, and click **Add**. Claude discovers the authorization server,
registers itself, and opens `/authorize` — paste the `TALLY_MCP_TOKEN` value into the one-field
form to authorize. The server must be reachable over HTTPS at exactly `TALLY_PUBLIC_BASE_URL`.

### Verifying the connection

Once connected, ask the assistant to list ledgers (`list_ledgers`) — on a fresh database this
should return an empty list, confirming the round trip: client → bearer token auth → MCP
JSON-RPC → tally's query layer → SQLite. Create a ledger with `manage_ledger` before asking it to
list or record anything else.
