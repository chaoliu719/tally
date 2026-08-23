## 1. Storage schema

- [x] 1.1 In `internal/store/schema.sql`, replace the `accounts` table with a `sources` table (`id`/`name`/`created_at`/`updated_at` only); rename `transactions.account_id` to `source_id` (still `REFERENCES sources(id)`); add a `NOT NULL` `currency` column to `transactions`; drop `adjustment` from the type comment and the `CHECK` constraint's OR-branch, changing `category_id` to `NOT NULL`; rename the `idx_transactions_account_time` index to `idx_transactions_source_time`. Verify by running the server once against a fresh SQLite file and confirming the new tables/columns exist (`sqlite3 <file> .schema`).
- [x] 1.2 In `internal/store/queries.sql`, rename every `accounts`-related query (`CreateAccount`/`ListAccounts`/`GetAccount`/`UpdateAccount`/`DeleteAccount`/`CountTransactionsByAccount` or equivalents) to their `Source` equivalents, dropping `type`/`currency`/`comment` from inserts/selects/updates. Verify by reading the diff and confirming no `type`/`currency`/`comment` column reference remains for sources.
- [x] 1.3 In `internal/store/queries.sql`, update `CreateTransaction`/`UpdateTransaction`/`GetTransaction`/`SearchTransactions`/`DeleteTransaction` to use `source_id` and the new `currency` column; remove `adjustment` handling. Verify by reading the diff.
- [x] 1.4 In `internal/store/queries.sql`, rewrite `SummarizeTransactionsByCurrency` to group by `t.currency` (drop the `JOIN accounts`, drop the `adjustment` output column); rewrite `SummarizeTransactionsByCategory` to group by `t.category_id, t.currency` (drop the `JOIN accounts` and the now-unnecessary `WHERE t.type IN ('income','expense')`); rename `SummarizeTransactionsByAccount` to `SummarizeTransactionsBySource`, grouping by `t.source_id, t.currency` the same way. Verify by reading the diff.
- [x] 1.5 Regenerate sqlc output (`internal/store/queries.sql.go`, `internal/store/querier.go`) and verify `go build ./...` succeeds.

## 2. Source management tool (replaces account management)

- [x] 2.1 Rename `internal/tools/accounts.go` to `internal/tools/sources.go`; rename `list_accounts`/`manage_account` tools to `list_sources`/`manage_source`; drop `type`/`currency`/`comment`/`balance` from input/output structs, keeping only `id`/`name`. Verify with `go build ./internal/tools`.
- [x] 2.2 Update `manage_source`'s `operation="delete"` preview → apply flow to reuse the existing reference-check logic (blocked if any transaction references the source), same as `manage_category`. Verify by adapting the existing account-delete unit tests in `internal/tools/accounts_test.go` (renamed to `sources_test.go`) to the new shape and running `go test ./internal/tools/...`.
- [x] 2.3 Remove the old `type`/`currency` validation from source create/update; confirm currency validation now lives only on transaction creation/update (task 3.x). Verify by grepping `internal/tools/sources.go` for `currency` and confirming no hits remain there.

## 3. Transaction recording changes

- [x] 3.1 In `internal/tools/transactions.go`, rename `AccountID`/`account_id` to `SourceID`/`source_id` across `CreateTransactionInput`/`UpdateTransactionInput`/`TransactionInfo`/`SearchTransactionsInput` and their resolution logic. Verify with `go build ./internal/tools`.
- [x] 3.2 Add a required `currency` field to `CreateTransactionInput`/`UpdateTransactionInput`, validated against the existing static ISO 4217 table (same validation previously applied to account creation); `TransactionInfo`'s `currency` output now comes directly from the stored transaction instead of a joined source. Verify with a new unit test asserting `create_transaction` rejects an unsupported currency code.
- [x] 3.3 Remove the `adjustment` transaction type entirely: delete the "record a balance adjustment" code path in `create_transaction`/`update_transaction`, the `category_id` nullability special-case, and any adjustment-specific error messages. Verify by running `go test ./internal/tools/...` and confirming no reference to `adjustment` remains (`grep -ri adjustment internal/`).
- [x] 3.4 Update `internal/tools/transactions_test.go` (and `transactions_cursor_test.go` if it references `account_id`) to use `source_id`/`currency` and drop all adjustment-related test cases; add cases for the new required-currency and unsupported-currency scenarios from `transaction-recording`'s delta spec. Verify with `go test ./internal/tools/...`.

## 4. Financial analytics changes

- [x] 4.1 In `internal/tools/analytics.go`, rename the by-account breakdown to by-source (`AccountID`/`account_id` → `SourceID`/`source_id` in the summary output struct), and remove the separate adjustment-total field from `get_financial_summary`'s output. Verify with `go build ./internal/tools`.
- [x] 4.2 Update `internal/tools/analytics_test.go` to match: rename account fixtures/assertions to source, remove adjustment-total assertions, keep currency-grouping assertions (now driven by `t.currency` instead of the source). Verify with `go test ./internal/tools/...`.

## 5. End-to-end tests and docs

- [x] 5.1 Rename `cmd/tally-mcp/e2e_account_lifecycle_test.go` to `e2e_source_lifecycle_test.go` and update it to exercise `list_sources`/`manage_source` without type/currency/balance fields. Verify with `go test ./cmd/tally-mcp/...`.
- [x] 5.2 Update `cmd/tally-mcp/e2e_transaction_lifecycle_test.go` and `e2e_minimal_loop_test.go` to use `source_id`, pass an explicit `currency` on every `create_transaction`/`update_transaction` call, and drop every `adjustment`-type scenario. Verify with `go test ./cmd/tally-mcp/...`.
- [x] 5.3 Update `README.md`: replace every `list_accounts`/`manage_account`/`account_id`/"account" reference with the source equivalents, drop the `adjustment` transaction type from the `create_transaction` row and the `get_financial_summary` row, and mention the new required `currency` parameter on `create_transaction`/`update_transaction`. Verify by re-reading the file for any remaining "account" or "adjustment" mention outside historical/unrelated context.
- [x] 5.4 Run the full test suite (`go test ./...`) and confirm it passes end to end.

## 6. Spec sync

- [x] 6.1 Once implementation is verified, run the sync/archive workflow to merge this change's delta specs into `openspec/specs/` and archive the change directory.
