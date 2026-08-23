package tools_test

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/tools"
)

func futureTime() int64 {
	return time.Now().Add(time.Hour).Unix()
}

// setupSourceAndCategory creates a source and a usable category, returning
// their ids. Categories no longer have a level restriction, so a plain
// top-level category works fine here.
func setupSourceAndCategory(t *testing.T, session *mcp.ClientSession) (sourceID, categoryID string) {
	t.Helper()

	var source tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		Operation: "create",
		Name:      "Cash Wallet",
	}, &source)

	var category tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Groceries",
	}, &category)

	return source.Source.ID, category.Category.ID
}

func TestCreateTransactionExpense(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     2500,
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "groceries run",
	}, &created)

	if created.Transaction.Amount != 2500 {
		t.Errorf("Amount = %d, want 2500", created.Transaction.Amount)
	}
	if created.Transaction.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", created.Transaction.Currency)
	}
	if created.Transaction.SourceID != sourceID {
		t.Errorf("SourceID = %q, want %q", created.Transaction.SourceID, sourceID)
	}

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &got)
	if got.Transaction.ID != created.Transaction.ID {
		t.Errorf("get_transaction id = %q, want %q", got.Transaction.ID, created.Transaction.ID)
	}
}

func TestCreateTransactionIncome(t *testing.T) {
	session := newTestSession(t)
	sourceID, _ := setupSourceAndCategory(t, session)

	var incomeCategory tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Salary",
	}, &incomeCategory)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "income",
		SourceID:   sourceID,
		CategoryID: incomeCategory.Category.ID,
		Amount:     5000,
		Currency:   "CNY",
		Time:       futureTime(),
	}, &created)

	if created.Transaction.Amount != 5000 {
		t.Errorf("Amount = %d, want 5000", created.Transaction.Amount)
	}
}

func TestCreateTransactionRejectsNonexistentSource(t *testing.T) {
	session := newTestSession(t)
	_, categoryID := setupSourceAndCategory(t, session)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   "999999",
		CategoryID: categoryID,
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

func TestCreateTransactionRejectsNonexistentCategory(t *testing.T) {
	session := newTestSession(t)
	sourceID, _ := setupSourceAndCategory(t, session)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: "999999",
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

// TestCreateTransactionAllowsTopLevelCategory verifies the former "must be a
// second-level category" restriction is gone: a plain top-level category
// (no parent) can now be referenced directly by create_transaction.
func TestCreateTransactionAllowsTopLevelCategory(t *testing.T) {
	session := newTestSession(t)
	sourceID, _ := setupSourceAndCategory(t, session)

	var topLevel tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Housing",
	}, &topLevel)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: topLevel.Category.ID,
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})
}

func TestCreateTransactionMissingRequiredField(t *testing.T) {
	session := newTestSession(t)
	sourceID, _ := setupSourceAndCategory(t, session)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:     "expense",
		SourceID: sourceID,
		// CategoryID omitted
		Amount:   100,
		Currency: "CNY",
		Time:     futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

func TestCreateTransactionMissingCurrency(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     100,
		// Currency omitted
		Time: futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

func TestCreateTransactionUnsupportedCurrency(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     100,
		Currency:   "NOTACURRENCY",
		Time:       futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

func TestGetTransactionNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{ID: "999999"})
}

func TestSearchTransactionsNoFilter(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 200, Currency: "CNY", Time: futureTime() + 3600,
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &out)
	if len(out.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(out.Transactions))
	}
}

func TestSearchTransactionsTimeRange(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	earlyTime := futureTime()
	lateTime := earlyTime + 100000

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: earlyTime,
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 200, Currency: "CNY", Time: lateTime,
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		StartTime: earlyTime - 10,
		EndTime:   earlyTime + 10,
	}, &out)

	if len(out.Transactions) != 1 {
		t.Fatalf("expected 1 transaction in range, got %d", len(out.Transactions))
	}
	if out.Transactions[0].Amount != 100 {
		t.Errorf("Amount = %d, want 100", out.Transactions[0].Amount)
	}
}

func TestUpdateTransactionHappyPath(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     2500,
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "original",
	}, &created)

	var otherCategory tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		Operation: "create",
		Name:      "Dining",
	}, &otherCategory)

	var updated tools.UpdateTransactionOutput
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: otherCategory.Category.ID,
		Amount:     4000,
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "revised",
	}, &updated)

	if updated.Transaction.Amount != 4000 {
		t.Errorf("Amount = %d, want 4000", updated.Transaction.Amount)
	}
	if updated.Transaction.CategoryID != otherCategory.Category.ID {
		t.Errorf("CategoryID = %q, want %q", updated.Transaction.CategoryID, otherCategory.Category.ID)
	}
	if updated.Transaction.Comment != "revised" {
		t.Errorf("Comment = %q, want %q", updated.Transaction.Comment, "revised")
	}

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != 4000 {
		t.Errorf("get_transaction Amount = %d, want 4000", got.Transaction.Amount)
	}
}

func TestUpdateTransactionChangesSource(t *testing.T) {
	session := newTestSession(t)
	sourceA, categoryID := setupSourceAndCategory(t, session)

	var sourceB tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		Operation: "create",
		Name:      "Second Wallet",
	}, &sourceB)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type:       "expense",
		SourceID:   sourceA,
		CategoryID: categoryID,
		Amount:     2000,
		Currency:   "CNY",
		Time:       futureTime(),
	}, &created)

	var updated tools.UpdateTransactionOutput
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceB.Source.ID,
		CategoryID: categoryID,
		Amount:     2000,
		Currency:   "CNY",
		Time:       futureTime(),
	}, &updated)

	if updated.Transaction.SourceID != sourceB.Source.ID {
		t.Errorf("SourceID = %q, want %q", updated.Transaction.SourceID, sourceB.Source.ID)
	}
}

func TestUpdateTransactionMissingRequiredField(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:   created.Transaction.ID,
		Type: "expense",
		// SourceID omitted
		CategoryID: categoryID,
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != 100 {
		t.Fatalf("transaction changed after rejected update: Amount = %d, want 100", got.Transaction.Amount)
	}
}

func TestUpdateTransactionRejectsNonexistentSource(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   "999999",
		CategoryID: categoryID,
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestUpdateTransactionRejectsNonexistentCategory(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: "999999",
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestUpdateTransactionRejectsUnsupportedCurrency(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     100,
		Currency:   "NOTACURRENCY",
		Time:       futureTime(),
	})
}

func TestUpdateTransactionIncomeExpenseRejectsNonPositiveAmount(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     0,
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestUpdateTransactionNotFound(t *testing.T) {
	session := newTestSession(t)
	_, categoryID := setupSourceAndCategory(t, session)
	sourceID, _ := setupSourceAndCategory(t, session)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         "999999",
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     100,
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestDeleteTransactionHappyPath(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 2500, Currency: "CNY", Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: created.Transaction.ID}, &preview)
	if preview.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", preview.Status, "pending_confirmation")
	}
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}

	// Preview must not delete anything.
	var stillThere tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &stillThere)

	var applied tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)
	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &list)
	for _, txn := range list.Transactions {
		if txn.ID == created.Transaction.ID {
			t.Fatalf("deleted transaction %q still present in search_transactions", created.Transaction.ID)
		}
	}
}

func TestDeleteTransactionNotFound(t *testing.T) {
	session := newTestSession(t)

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: "999999"})
	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: "999999", ConfirmationToken: "irrelevant"})
}

func TestDeleteTransactionExpiredToken(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	expired := craftConfirmationToken(t, testConfirmSecret, "delete_transaction", created.Transaction.ID, "irrelevant-revision", time.Now().Add(-time.Minute).Unix())

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: expired,
	})

	// The transaction must survive the expired token.
	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestDeleteTransactionDriftedRevisionAfterUpdate(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: created.Transaction.ID}, &preview)

	// The transaction changes after the preview but before apply -- the stale
	// token must be rejected rather than deleting a transaction whose state no
	// longer matches what was previewed.
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     200,
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.UpdateTransactionOutput{})

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	})

	callTool(t, session, "get_transaction", tools.GetTransactionInput{ID: created.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestDeleteTransactionTokenReplay(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{ID: created.Transaction.ID}, &preview)

	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &tools.DeleteTransactionOutput{})

	// Reusing the same token to confirm the same delete a second time must
	// fail -- the transaction is already gone.
	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	})
}

func TestSearchTransactionsEmptyResult(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 100, Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		StartTime: futureTime() + 1000000,
		EndTime:   futureTime() + 1100000,
	}, &out)

	if out.Transactions == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Transactions) != 0 {
		t.Fatalf("expected 0 transactions, got %d", len(out.Transactions))
	}
	if out.NextCursor != "" {
		t.Fatalf("expected no next_cursor for an empty result, got %q", out.NextCursor)
	}
}

// createExpensesAtOffsets creates one expense transaction per offset in
// offsets, at time base+offset, and returns their ids in the same order as
// offsets. Offsets should be distinct and increasing so time alone
// determines a deterministic order.
func createExpensesAtOffsets(t *testing.T, session *mcp.ClientSession, sourceID, categoryID string, base int64, offsets []int64) []string {
	t.Helper()
	ids := make([]string, 0, len(offsets))
	for i, off := range offsets {
		var created tools.CreateTransactionOutput
		callTool(t, session, "create_transaction", tools.CreateTransactionInput{
			Type:       "expense",
			SourceID:   sourceID,
			CategoryID: categoryID,
			Amount:     int64(i + 1),
			Currency:   "CNY",
			Time:       base + off,
		}, &created)
		ids = append(ids, created.Transaction.ID)
	}
	return ids
}

// createNExpenses creates n expense transactions at times baseTime+1,
// baseTime+2, ..., baseTime+n (strictly increasing, so ordering by time
// alone is deterministic) and returns their ids in creation order.
func createNExpenses(t *testing.T, session *mcp.ClientSession, sourceID, categoryID string, n int, baseTime int64) []string {
	t.Helper()
	offsets := make([]int64, n)
	for i := range offsets {
		offsets[i] = int64(i + 1)
	}
	return createExpensesAtOffsets(t, session, sourceID, categoryID, baseTime, offsets)
}

// TestSearchTransactionsDefaultPageSize verifies the default page size is 50
// (proposal.md / spec.md: "无筛选条件" scenario -- returns the oldest page,
// not everything) by creating more than 50 matching transactions and calling
// search_transactions with limit omitted.
func TestSearchTransactionsDefaultPageSize(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)
	createNExpenses(t, session, sourceID, categoryID, 55, futureTime())

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{}, &out)

	if len(out.Transactions) != 50 {
		t.Fatalf("expected the default page size of 50 transactions, got %d", len(out.Transactions))
	}
	if out.NextCursor == "" {
		t.Fatal("expected next_cursor to be present since more than 50 transactions match")
	}
}

// TestSearchTransactionsNextCursorOnlyWhenMoreResults covers both halves of
// spec.md's "结果超过一页" scenario: next_cursor is present when there's
// another page, and absent when the current page is the last one.
func TestSearchTransactionsNextCursorOnlyWhenMoreResults(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)
	createNExpenses(t, session, sourceID, categoryID, 5, futureTime())

	var exactFit tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 5}, &exactFit)
	if len(exactFit.Transactions) != 5 {
		t.Fatalf("expected 5 transactions, got %d", len(exactFit.Transactions))
	}
	if exactFit.NextCursor != "" {
		t.Fatalf("expected no next_cursor when limit exactly fits all results, got %q", exactFit.NextCursor)
	}

	var shortOfFit tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 4}, &shortOfFit)
	if len(shortOfFit.Transactions) != 4 {
		t.Fatalf("expected 4 transactions, got %d", len(shortOfFit.Transactions))
	}
	if shortOfFit.NextCursor == "" {
		t.Fatal("expected next_cursor to be present when 5 results don't fit in a page of 4")
	}
}

// TestSearchTransactionsCursorPaginationCoversAllResults covers spec.md's
// "使用 cursor 翻页" scenario: paging through with a small limit until
// next_cursor disappears yields exactly the same set (no duplicates, no
// omissions, same order) as fetching everything in one page.
func TestSearchTransactionsCursorPaginationCoversAllResults(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)
	createNExpenses(t, session, sourceID, categoryID, 12, futureTime())

	var full tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 200}, &full)
	if len(full.Transactions) != 12 {
		t.Fatalf("expected 12 transactions unpaginated, got %d", len(full.Transactions))
	}

	var paged []tools.TransactionInfo
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("pagination did not terminate")
		}
		var page tools.SearchTransactionsOutput
		callTool(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 5, Cursor: cursor}, &page)
		paged = append(paged, page.Transactions...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	if len(paged) != len(full.Transactions) {
		t.Fatalf("paginated total = %d, want %d", len(paged), len(full.Transactions))
	}
	seen := map[string]bool{}
	for i, txn := range paged {
		if seen[txn.ID] {
			t.Fatalf("transaction %q returned more than once across pages", txn.ID)
		}
		seen[txn.ID] = true
		if txn.ID != full.Transactions[i].ID {
			t.Fatalf("paged[%d].ID = %q, want %q (order must match the unpaginated result)", i, txn.ID, full.Transactions[i].ID)
		}
	}
}

// TestSearchTransactionsInvalidCursorRejected and
// TestSearchTransactionsLimitOverMaxRejected cover spec.md's "cursor 无效或
// 已不匹配当前筛选条件" and "limit 超过上限" scenarios.
func TestSearchTransactionsInvalidCursorRejected(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)
	createNExpenses(t, session, sourceID, categoryID, 3, futureTime())

	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{
		Cursor: "this-is-not-a-valid-cursor",
	})
}

func TestSearchTransactionsCursorRejectedWhenFiltersChange(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)
	createNExpenses(t, session, sourceID, categoryID, 3, futureTime())

	var otherSource tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		Operation: "create",
		Name:      "Other Wallet",
	}, &otherSource)

	var page tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 1}, &page)
	if page.NextCursor == "" {
		t.Fatal("expected next_cursor from the first page to exercise the mismatch")
	}

	// Same cursor, but now scoped to a different source_id filter than the
	// one it was issued under -- must be rejected, not silently reused.
	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{
		Limit:    1,
		SourceID: otherSource.Source.ID,
		Cursor:   page.NextCursor,
	})
}

func TestSearchTransactionsLimitOverMaxRejected(t *testing.T) {
	session := newTestSession(t)
	setupSourceAndCategory(t, session)

	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 201})
}

// TestSearchTransactionsPaginationSurvivesLedgerChanges covers spec.md's
// "上一页返回后账本发生变化" scenario: keyset pagination continues correctly
// from the cursor's recorded (time, id) position even after the ledger
// changes between calls -- new transactions are inserted (one before the
// cursor position, one after) and an already-returned transaction is
// updated. Not-yet-returned transactions must all still show up exactly
// once; the transaction inserted behind the cursor position is legitimately
// excluded, since keyset pagination cannot retroactively insert rows before
// a position already paged past.
func TestSearchTransactionsPaginationSurvivesLedgerChanges(t *testing.T) {
	session := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session)
	base := futureTime()

	// T1..T4 at base+10, +20, +30, +40.
	ids := createExpensesAtOffsets(t, session, sourceID, categoryID, base, []int64{10, 20, 30, 40})

	var page1 tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 2}, &page1)
	if len(page1.Transactions) != 2 {
		t.Fatalf("page1: expected 2 transactions, got %d", len(page1.Transactions))
	}
	if page1.NextCursor == "" {
		t.Fatal("page1: expected a next_cursor")
	}
	// page1 should be [T1, T2].
	if page1.Transactions[0].ID != ids[0] || page1.Transactions[1].ID != ids[1] {
		t.Fatalf("page1 = [%q, %q], want [%q, %q]", page1.Transactions[0].ID, page1.Transactions[1].ID, ids[0], ids[1])
	}

	// Ledger changes between page1 and page2:
	// - a new transaction inserted before the cursor position (base+5,
	//   earlier than T2's base+20, which page1's cursor already points
	//   past) -- legitimately excluded from all future pages, since keyset
	//   pagination can't retroactively insert rows before a position
	//   already paged past.
	var before tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 999, Currency: "CNY", Time: base + 5,
	}, &before)
	// - a new transaction inserted after everything so far -- must appear.
	var after tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		Type: "expense", SourceID: sourceID, CategoryID: categoryID, Amount: 998, Currency: "CNY", Time: base + 50,
	}, &after)
	// - T1 (already returned in page1) is updated -- must not reappear or
	//   otherwise disturb pagination. Time is left unchanged so its keyset
	//   position is stable.
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         ids[0],
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     12345,
		Currency:   "CNY",
		Time:       base + 10,
		Comment:    "edited after page1",
	}, &tools.UpdateTransactionOutput{})

	var rest []tools.TransactionInfo
	cursor := page1.NextCursor
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("pagination did not terminate")
		}
		var page tools.SearchTransactionsOutput
		callTool(t, session, "search_transactions", tools.SearchTransactionsInput{Limit: 2, Cursor: cursor}, &page)
		rest = append(rest, page.Transactions...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	wantIDs := []string{ids[2], ids[3], after.Transaction.ID} // T3, T4, "after"
	if len(rest) != len(wantIDs) {
		t.Fatalf("remaining pages returned %d transactions, want %d (%v got %v)", len(rest), len(wantIDs), idsOf(rest), wantIDs)
	}
	for i, txn := range rest {
		if txn.ID != wantIDs[i] {
			t.Fatalf("rest[%d].ID = %q, want %q (full sequence: %v)", i, txn.ID, wantIDs[i], idsOf(rest))
		}
	}
	for _, txn := range rest {
		if txn.ID == ids[0] {
			t.Fatal("T1 (already returned in page1) reappeared after being updated")
		}
		if txn.ID == before.Transaction.ID {
			t.Fatal("the transaction inserted behind the cursor position unexpectedly appeared")
		}
	}
}

func idsOf(txns []tools.TransactionInfo) []string {
	ids := make([]string, len(txns))
	for i, txn := range txns {
		ids[i] = txn.ID
	}
	return ids
}
