## 1. Schema and generated queries

- [x] 1.1 Remove the `type` column from `categories` in `internal/store/schema.sql`; verify `go build ./...` still compiles (schema is embedded, not migrated)
- [x] 1.2 In `internal/store/queries.sql`, add `UpdateAccount`, `DeleteAccount`, `CountTransactionsByAccount`; add `UpdateCategory`, `DeleteCategory`, `CountChildCategories`, `CountTransactionsByCategory`, and a `WITH RECURSIVE` query returning all descendant ids of a category; remove `type` from the existing category queries (`CreateCategory`, `GetCategory`, `ListCategories`)
- [x] 1.3 Regenerate sqlc output and verify `go build ./...` succeeds with the new generated methods available on `store.Queries`

## 2. Confirmation token package

- [x] 2.1 Create `internal/confirm` implementing `Issue(secret, action, id, revision string) (token string, expiresAt int64)` and `Verify(secret, token, wantAction, wantID, currentRevision string, now time.Time) error`, per [design.md](design.md)'s token format (base64url JSON payload + base64url HMAC-SHA256 signature, constant-time comparison)
- [x] 2.2 Add unit tests in `internal/confirm`: valid issue→verify round trip; tampered signature rejected; wrong `action` rejected; wrong `id` rejected; expired token rejected; mismatched `revision` rejected; verify each failure mode returns a distinct, descriptive error

## 3. Startup configuration

- [x] 3.1 Add `TALLY_CONFIRMATION_SECRET` to `internal/bootstrap/config.go`, required with fail-fast behavior mirroring `TALLY_MCP_TOKEN`; update `internal/bootstrap` tests to cover the new required variable

## 4. Account update and delete

- [x] 4.1 Add `Operation` (`create`/`update`/`delete`) and `ID` fields to `ManageAccountInput`; dispatch on `Operation` in `manageAccount`, rejecting unrecognized values; add `Status` to `ManageAccountOutput`
- [x] 4.2 Implement `operation="update"`: require `id`, `name`, `type`, `comment` together (full replace); reject if `currency` or nonzero `balance` is provided; reject if the account doesn't exist; return `status="updated"`
- [x] 4.3 Implement `operation="delete"` preview (no `confirmation_token`): compute the account's current revision (name/type/comment/currency + referencing transaction count) per design.md, reject with an error if the transaction count is nonzero, otherwise issue a token via `internal/confirm` and return `status="pending_confirmation"` with the token and its expiry
- [x] 4.4 Implement `operation="delete"` apply (`confirmation_token` present): verify the token via `internal/confirm`, then inside one `sql.Tx` re-check the transaction count is still zero before deleting the account row; return `status="deleted"` with the account's last-known info; verify the account no longer appears in `list_accounts`
- [x] 4.5 Update `internal/tools/accounts_test.go`: adjust existing create-path tests for the now-required `Operation` field; add tests for each scenario in `specs/account-management/spec.md`'s MODIFIED/ADDED requirements (update happy path, missing fields, currency/balance rejection, delete happy path, delete blocked by references, delete of nonexistent account, preview/apply token flow, expired token, drifted revision)

## 5. Category model simplification and update/delete

- [x] 5.1 Remove `Type` from `CategoryInfo`, `ManageCategoryInput`; remove the `categoryTypes` whitelist and `topLevelParentID`-vs-second-level validation in `internal/tools/categories.go`; `create_transaction`'s check that a category must be second-level is removed as part of this (see task 6.1)
- [x] 5.2 Add `Operation` and `ID` fields to `ManageCategoryInput`; dispatch on `Operation`, rejecting unrecognized values; add `Status` to `ManageCategoryOutput`; update `operation="create"` to allow `parent_id` pointing at any existing category (not just a top-level one)
- [x] 5.3 Implement `operation="update"`: require `id`, `name`, `parent_id` together (full replace); reject self-reference (`parent_id == id`); reject cycles using the descendant-ids query from task 1.2; reject if the target category or the new parent doesn't exist; return `status="updated"`
- [x] 5.4 Implement `operation="delete"` preview and apply mirroring 4.3/4.4: revision covers name/parent_id + child count + referencing transaction count; reject if either count is nonzero; apply re-checks both counts inside the same `sql.Tx` as the delete
- [x] 5.5 Update `internal/tools/categories_test.go`: remove/replace `type`-related assertions; add tests for each scenario in `specs/category-management/spec.md` (arbitrary-depth create, update name/move, self-reference rejection, cycle rejection, delete blocked by children, delete blocked by references, preview/apply flow)

## 6. Transaction recording: balance_adjustment

- [x] 6.1 In `internal/tools/transactions.go`, add `balance_adjustment` to `createableTransactionTypes`; branch validation so `balance_adjustment` requires a nonzero `amount` and rejects a nonempty `category_id`, while `income`/`expense` keep requiring a valid existing category (any category, since 5.1 removed the second-level restriction) and a positive `amount`
- [x] 6.2 Update `internal/tools/transactions_test.go`: add tests for each scenario in `specs/transaction-recording/spec.md` (balance_adjustment happy path with positive and negative amounts, category_id rejected, zero amount rejected, nonexistent account rejected); update the existing income/expense category tests to no longer assume a two-level requirement

## 7. Docs and validation

- [x] 7.1 Update `README.md`: the Tools table (`manage_account`/`manage_category`/`create_transaction` descriptions and the two-level category explanation around line 23), and the environment variables table (add `TALLY_CONFIRMATION_SECRET`)
- [x] 7.2 Run `openspec validate account-category-lifecycle --strict` and fix any reported issues

## 8. End-to-end verification

- [x] 8.1 Run `go build ./...` and `go test ./...`, confirm everything passes
- [x] 8.2 Start the server locally with both required env vars set and, using an MCP client (or the project's documented e2e approach), walk through: create → update → preview-delete → apply-delete for an account; create a nested category tree, move a category, delete a leaf category; record a `balance_adjustment` transaction and confirm the account balance reflects it
