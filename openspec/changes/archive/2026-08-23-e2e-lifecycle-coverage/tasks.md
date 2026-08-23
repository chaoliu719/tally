## 1. Internal-layer replay-token coverage

- [x] 1.1 Add `TestManageAccountDeleteTokenReplay` to `internal/tools/accounts_test.go`: delete an account via preview → apply (reusing the pattern from `TestManageAccountDeleteHappyPath`), then call `operation="delete"` again with the same `confirmation_token`; verify the second call errors via `callToolExpectError` and `list_accounts` still shows zero accounts
- [x] 1.2 Add `TestManageCategoryDeleteTokenReplay` to `internal/tools/categories_test.go`: same shape for a category, verifying the second call errors and `list_categories` still shows zero categories
- [x] 1.3 Run `go test ./internal/tools/...` and confirm both new tests pass

## 2. Shared e2e session helper

- [x] 2.1 Create `cmd/tally-mcp/e2e_testutil_test.go`: extract the `bootstrap.Config`/`InitDataStore`/`buildMux`/`httptest.NewServer`/MCP-client-connect setup currently inlined in `TestEndToEndMinimalLoop`, plus the `call` helper and `futureTime`, into a reusable `newE2ESession(t *testing.T) *mcp.ClientSession` (with `t.Cleanup` for teardown) and a package-level `call`/`callExpectError` pair
- [x] 2.2 Rename `cmd/tally-mcp/e2e_test.go` to `cmd/tally-mcp/e2e_minimal_loop_test.go` and rename `TestEndToEndMinimalLoop` to `TestE2EMinimalLoop`, to match the `e2e_<scope>_test.go` / `TestE2E<Scope>` pattern used by the new files in this change; update it to use the extracted helpers instead of its inline setup; verify `go test ./cmd/tally-mcp/...` still passes with no behavior change

## 3. Account lifecycle e2e journey

- [x] 3.1 Create `cmd/tally-mcp/e2e_account_lifecycle_test.go` with `TestE2EAccountLifecycle`, using two accounts (see design.md — `balance_adjustment` permanently blocks delete under the current implementation, so the two capabilities can't share one account): account A goes create → update (`name`/`type`/`comment`, verified via `list_accounts`) → preview-delete (assert `pending_confirmation` + token) → apply-delete (assert `deleted` and gone from `list_accounts`), never recording any transaction against it; account B goes create → record a `balance_adjustment` transaction → verify the balance change via `list_accounts`, and is never deleted
- [x] 3.2 Run `go test ./cmd/tally-mcp/... -run TestE2EAccountLifecycle` and confirm it passes

## 4. Category lifecycle e2e journey

- [x] 4.1 Create `cmd/tally-mcp/e2e_category_lifecycle_test.go` with `TestE2ECategoryLifecycle`: using `newE2ESession`, create a three-level nested category tree, move a node to a different parent and verify via `list_categories`, then delete a leaf category through preview → apply and verify it is gone from `list_categories`
- [x] 4.2 Run `go test ./cmd/tally-mcp/... -run TestE2ECategoryLifecycle` and confirm it passes

## 5. Full verification

- [x] 5.1 Run `go build ./...` and `go test ./...`, confirm everything passes with no regressions to existing tests
