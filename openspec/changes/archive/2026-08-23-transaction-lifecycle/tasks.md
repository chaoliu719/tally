## 1. Schema and generated queries

- [x] 1.1 Add `updated_at INTEGER NOT NULL` to the `transactions` table in `internal/store/schema.sql`; verify `go build ./...` still compiles (schema is embedded, not migrated)
- [x] 1.2 In `internal/store/queries.sql`: add `UpdateTransaction` (`:one`, sets `type`/`account_id`/`category_id`/`amount`/`time`/`comment`/`updated_at`, returns the updated row) and `DeleteTransaction` (`:exec`); update `CreateTransaction` to also set `updated_at` equal to `created_at` on insert
- [x] 1.3 Regenerate sqlc output and verify `go build ./...` succeeds with `UpdateTransaction`/`DeleteTransaction` available on `store.Queries`

## 2. Shared field validation

- [x] 2.1 In `internal/tools/transactions.go`, extract `createTransaction`'s field validation (type whitelist, account existence, category existence/nullness per type, amount sign/nonzero rules per type) into a helper reusable by both `createTransaction` and `updateTransaction`; verify existing `create_transaction` tests in `transactions_test.go` still pass unchanged after the refactor

## 3. `update_transaction`

- [x] 3.1 Add `UpdateTransactionInput` (mirrors `CreateTransactionInput` plus required `ID`) and `UpdateTransactionOutput` (`Transaction TransactionInfo`); register the `update_transaction` tool in `registerTransactionTools` per [design.md](design.md)
- [x] 3.2 Implement `updateTransaction`: require `id`, look up the existing transaction (not-found error if missing), run the shared validation helper from 2.1, call `store.UpdateTransaction`, return the refreshed `TransactionInfo`
- [x] 3.3 Add tests in `transactions_test.go` for each scenario in `specs/transaction-recording/spec.md`'s "更新一笔交易" requirement: happy-path update including a changed `account_id` (verify both the old and new account's balance via `GetAccountBalance`), missing required fields, nonexistent account/category, `balance_adjustment` with `category_id` or zero `amount` rejected, `income`/`expense` with non-positive `amount` or missing `category_id` rejected, updating a nonexistent transaction id rejected

## 4. `delete_transaction`

- [x] 4.1 Add `confirmActionDeleteTransaction = "delete_transaction"` constant and a `transactionDeletionRevision(t store.Transaction) string` function (JSON-encode `Type`/`AccountID`/`CategoryID`/`Amount`/`Time`/`Comment`, sha256 hex) in `internal/tools/transactions.go`, mirroring `accountDeletionRevision` in `internal/tools/accounts.go`
- [x] 4.2 Add `DeleteTransactionInput` (`ID`, `ConfirmationToken`) and `DeleteTransactionOutput` (`Transaction TransactionInfo`, `Status`, `ConfirmationToken`, `ExpiresAt`); register the `delete_transaction` tool
- [x] 4.3 Implement the preview path (no `confirmation_token`): fetch the transaction (not-found error if missing), issue a token via `internal/confirm` with the revision from 4.1, return `status="pending_confirmation"`
- [x] 4.4 Implement the apply path (`confirmation_token` present): `confirm.Verify` against the current revision, then inside one `sql.Tx` re-fetch the transaction (reject if it no longer exists or its revision has drifted), call `store.DeleteTransaction`, commit, return `status="deleted"` with the transaction's last-known info
- [x] 4.5 Add tests in `transactions_test.go` for each scenario in `specs/transaction-recording/spec.md`'s "删除一笔交易" requirement: preview/apply happy path (verify the account's balance changes and `get_transaction`/`search_transactions` no longer show it), preview or apply on a nonexistent id, expired token, revision mismatch after the transaction was modified by `update_transaction` or already deleted, reusing an already-consumed token

## 5. Docs and validation

- [x] 5.1 Update `README.md`'s Tools table with `update_transaction` and `delete_transaction` rows
- [x] 5.2 Run `openspec validate transaction-lifecycle --strict` and fix any reported issues

## 6. End-to-end verification

- [x] 6.1 Add an integration/e2e test (alongside the style used in `openspec/changes/archive/2026-08-23-e2e-lifecycle-coverage/`) that walks through the previously-blocked path: create an account, record one or more transactions against it (including a `balance_adjustment`), delete every one of them via `delete_transaction` preview→apply, then confirm `manage_account`(`operation="delete"`) now completes its own preview→apply successfully; repeat the same path for a category via `manage_category`
- [x] 6.2 Add a test verifying account balance consistency: after `update_transaction` changes a transaction's `amount`, `type`, or `account_id`, and after `delete_transaction` removes a transaction, `GetAccountBalance`/`list_accounts` reflect the correct recomputed balance for every affected account
- [x] 6.3 Run `go build ./...` and `go test ./...`, confirm everything passes
