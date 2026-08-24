package tools_test

import (
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"tally/internal/currency"
	"tally/internal/tools"
)

func futureTime() int64 {
	return time.Now().Add(time.Hour).Unix()
}

// cnyAmount formats minorUnits (fen) as the CNY decimal-string amount the
// wire format now uses, so existing test cases keep exercising the same
// underlying minor-unit values they always did (e.g. cnyAmount(2500) ==
// "25.00").
func cnyAmount(minorUnits int64) string {
	s, err := currency.FormatMajor("CNY", minorUnits)
	if err != nil {
		panic(err)
	}
	return s
}

// setupSourceAndCategory creates a source and a usable category, returning
// their ids. Categories no longer have a level restriction, so a plain
// top-level category works fine here.
func setupSourceAndCategory(t *testing.T, session *mcp.ClientSession, ledgerID string) (sourceID, categoryID string) {
	t.Helper()

	var source tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Cash Wallet",
	}, &source)

	var category tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Groceries",
	}, &category)

	return source.Source.ID, category.Category.ID
}

func TestCreateTransactionExpense(t *testing.T) {
	session, ledgerID, q := newTestSessionWithStore(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(2500),
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "groceries run",
	}, &created)

	if created.Transaction.Amount != cnyAmount(2500) {
		t.Errorf("Amount = %q, want %q", created.Transaction.Amount, cnyAmount(2500))
	}
	if created.Transaction.Currency != "CNY" {
		t.Errorf("Currency = %q, want CNY", created.Transaction.Currency)
	}
	if created.Transaction.SourceID != sourceID {
		t.Errorf("SourceID = %q, want %q", created.Transaction.SourceID, sourceID)
	}
	// Expense must be stored as a negative minor-units integer, so
	// SUM(amount) directly yields the net balance -- verified independently
	// of the (always-positive) wire-format amount string.
	if got := storedAmount(t, q, ledgerID, created.Transaction.ID); got != -2500 {
		t.Errorf("stored amount = %d, want -2500", got)
	}

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
	if got.Transaction.ID != created.Transaction.ID {
		t.Errorf("get_transaction id = %q, want %q", got.Transaction.ID, created.Transaction.ID)
	}
}

func TestCreateTransactionIncome(t *testing.T) {
	session, ledgerID, q := newTestSessionWithStore(t)
	sourceID, _ := setupSourceAndCategory(t, session, ledgerID)

	var incomeCategory tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Salary",
	}, &incomeCategory)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "income",
		SourceID:   sourceID,
		CategoryID: incomeCategory.Category.ID,
		Amount:     cnyAmount(5000),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &created)

	if created.Transaction.Amount != cnyAmount(5000) {
		t.Errorf("Amount = %q, want %q", created.Transaction.Amount, cnyAmount(5000))
	}
	// Income must be stored as a positive minor-units integer.
	if got := storedAmount(t, q, ledgerID, created.Transaction.ID); got != 5000 {
		t.Errorf("stored amount = %d, want 5000", got)
	}
}

func TestCreateTransactionRejectsNonexistentSource(t *testing.T) {
	session, ledgerID := newTestSession(t)
	_, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   "999999",
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

func TestCreateTransactionRejectsNonexistentCategory(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, _ := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: "999999",
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

// TestCreateTransactionAllowsTopLevelCategory verifies the former "must be a
// second-level category" restriction is gone: a plain top-level category
// (no parent) can now be referenced directly by create_transaction.
func TestCreateTransactionAllowsTopLevelCategory(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, _ := setupSourceAndCategory(t, session, ledgerID)

	var topLevel tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Housing",
	}, &topLevel)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: topLevel.Category.ID,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})
}

func TestCreateTransactionMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, _ := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense",
		SourceID: sourceID,
		// CategoryID omitted
		Amount:   cnyAmount(100),
		Currency: "CNY",
		Time:     futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

func TestCreateTransactionMissingCurrency(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		// Currency omitted
		Time: futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

func TestCreateTransactionUnsupportedCurrency(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "NOTACURRENCY",
		Time:       futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

// TestCreateTransactionRejectsInvalidAmountFormat covers spec.md's "金额格式
// 非法" scenario: an amount string that isn't a valid decimal number is
// rejected, not silently coerced.
func TestCreateTransactionRejectsInvalidAmountFormat(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	for _, amount := range []string{"fifty", "50.0.0", "", "50,00"} {
		callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
			LedgerID:   ledgerID,
			Type:       "expense",
			SourceID:   sourceID,
			CategoryID: categoryID,
			Amount:     amount,
			Currency:   "CNY",
			Time:       futureTime(),
		})
	}

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

// TestCreateTransactionRejectsAmountPrecisionExceedingCurrency covers
// spec.md's "金额小数位数超出币种精度" scenario: CNY allows at most 2
// fractional digits, so "50.001" must be rejected.
func TestCreateTransactionRejectsAmountPrecisionExceedingCurrency(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     "50.001",
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

// TestCreateTransactionRejectsZeroOrNegativeAmount covers spec.md's "金额为
// 零或负值" scenario.
func TestCreateTransactionRejectsZeroOrNegativeAmount(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	for _, amount := range []string{"0.00", "-10.00"} {
		callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
			LedgerID:   ledgerID,
			Type:       "expense",
			SourceID:   sourceID,
			CategoryID: categoryID,
			Amount:     amount,
			Currency:   "CNY",
			Time:       futureTime(),
		})
	}

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	if len(list.Transactions) != 0 {
		t.Fatalf("expected no transaction recorded, got %d", len(list.Transactions))
	}
}

// TestCreateTransactionNonCNYCurrencyPrecision covers spec.md's amount
// precision guarantee end to end for non-CNY currencies: create_transaction
// -> get_transaction -> search_transactions must all agree on the exact
// amount string, at that currency's own standard precision -- not silently
// degraded to two decimal places the way a hardcoded "%.2f" formatter would.
func TestCreateTransactionNonCNYCurrencyPrecision(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	cases := []struct {
		name     string
		currency string
		amount   string
	}{
		{"JPY zero-decimal", "JPY", "5000"},
		{"BHD three-decimal", "BHD", "5.000"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var created tools.CreateTransactionOutput
			callTool(t, session, "create_transaction", tools.CreateTransactionInput{
				LedgerID:   ledgerID,
				Type:       "expense",
				SourceID:   sourceID,
				CategoryID: categoryID,
				Amount:     c.amount,
				Currency:   c.currency,
				Time:       futureTime(),
			}, &created)

			if created.Transaction.Amount != c.amount {
				t.Fatalf("create_transaction Amount = %q, want %q", created.Transaction.Amount, c.amount)
			}
			if created.Transaction.Currency != c.currency {
				t.Fatalf("create_transaction Currency = %q, want %q", created.Transaction.Currency, c.currency)
			}

			var got tools.GetTransactionOutput
			callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
			if got.Transaction.Amount != c.amount {
				t.Fatalf("get_transaction Amount = %q, want %q", got.Transaction.Amount, c.amount)
			}

			var searched tools.SearchTransactionsOutput
			callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, CategoryID: categoryID}, &searched)
			var found bool
			for _, txn := range searched.Transactions {
				if txn.ID != created.Transaction.ID {
					continue
				}
				found = true
				if txn.Amount != c.amount {
					t.Fatalf("search_transactions Amount = %q, want %q", txn.Amount, c.amount)
				}
			}
			if !found {
				t.Fatalf("search_transactions did not return the created transaction %q", created.Transaction.ID)
			}
		})
	}
}

func TestGetTransactionNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: "999999"})
}

func TestSearchTransactionsNoFilter(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime() + 3600,
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &out)
	if len(out.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(out.Transactions))
	}
}

func TestSearchTransactionsTimeRange(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	earlyTime := futureTime()
	lateTime := earlyTime + 100000

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: earlyTime,
	}, &tools.CreateTransactionOutput{})
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: lateTime,
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID:  ledgerID,
		StartTime: earlyTime - 10,
		EndTime:   earlyTime + 10,
	}, &out)

	if len(out.Transactions) != 1 {
		t.Fatalf("expected 1 transaction in range, got %d", len(out.Transactions))
	}
	if out.Transactions[0].Amount != cnyAmount(100) {
		t.Errorf("Amount = %q, want %q", out.Transactions[0].Amount, cnyAmount(100))
	}
}

func TestUpdateTransactionHappyPath(t *testing.T) {
	session, ledgerID, q := newTestSessionWithStore(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(2500),
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "original",
	}, &created)

	var otherCategory tools.ManageCategoryOutput
	callTool(t, session, "manage_category", tools.ManageCategoryInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Dining",
	}, &otherCategory)

	var updated tools.UpdateTransactionOutput
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: otherCategory.Category.ID,
		Amount:     cnyAmount(4000),
		Currency:   "CNY",
		Time:       futureTime(),
		Comment:    "revised",
	}, &updated)

	if updated.Transaction.Amount != cnyAmount(4000) {
		t.Errorf("Amount = %q, want %q", updated.Transaction.Amount, cnyAmount(4000))
	}
	if updated.Transaction.CategoryID != otherCategory.Category.ID {
		t.Errorf("CategoryID = %q, want %q", updated.Transaction.CategoryID, otherCategory.Category.ID)
	}
	if updated.Transaction.Comment != "revised" {
		t.Errorf("Comment = %q, want %q", updated.Transaction.Comment, "revised")
	}

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != cnyAmount(4000) {
		t.Errorf("get_transaction Amount = %q, want %q", got.Transaction.Amount, cnyAmount(4000))
	}
	if gotStored := storedAmount(t, q, ledgerID, created.Transaction.ID); gotStored != -4000 {
		t.Errorf("stored amount = %d, want -4000", gotStored)
	}
}

func TestUpdateTransactionChangesSource(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceA, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var sourceB tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Second Wallet",
	}, &sourceB)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerID,
		Type:       "expense",
		SourceID:   sourceA,
		CategoryID: categoryID,
		Amount:     cnyAmount(2000),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &created)

	var updated tools.UpdateTransactionOutput
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceB.Source.ID,
		CategoryID: categoryID,
		Amount:     cnyAmount(2000),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &updated)

	if updated.Transaction.SourceID != sourceB.Source.ID {
		t.Errorf("SourceID = %q, want %q", updated.Transaction.SourceID, sourceB.Source.ID)
	}
}

func TestUpdateTransactionMissingRequiredField(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID: ledgerID,
		ID:       created.Transaction.ID,
		Type:     "expense",
		// SourceID omitted
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != cnyAmount(100) {
		t.Fatalf("transaction changed after rejected update: Amount = %q, want %q", got.Transaction.Amount, cnyAmount(100))
	}
}

func TestUpdateTransactionRejectsNonexistentSource(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   "999999",
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestUpdateTransactionRejectsNonexistentCategory(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: "999999",
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestUpdateTransactionRejectsUnsupportedCurrency(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "NOTACURRENCY",
		Time:       futureTime(),
	})
}

func TestUpdateTransactionIncomeExpenseRejectsNonPositiveAmount(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(0),
		Currency:   "CNY",
		Time:       futureTime(),
	})

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     "-10.00",
		Currency:   "CNY",
		Time:       futureTime(),
	})

	// The rejected updates above must not have changed the transaction.
	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != cnyAmount(100) {
		t.Fatalf("transaction changed after rejected updates: Amount = %q, want %q", got.Transaction.Amount, cnyAmount(100))
	}
}

// TestUpdateTransactionRejectsInvalidAmountFormat covers spec.md's "金额格式
// 非法" scenario for update_transaction, mirroring create_transaction's rule.
func TestUpdateTransactionRejectsInvalidAmountFormat(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     "fifty",
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != cnyAmount(100) {
		t.Fatalf("transaction changed after rejected update: Amount = %q, want %q", got.Transaction.Amount, cnyAmount(100))
	}
}

// TestUpdateTransactionRejectsAmountPrecisionExceedingCurrency covers
// spec.md's "金额小数位数超出币种精度" scenario for update_transaction.
func TestUpdateTransactionRejectsAmountPrecisionExceedingCurrency(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     "50.001",
		Currency:   "CNY",
		Time:       futureTime(),
	})

	var got tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &got)
	if got.Transaction.Amount != cnyAmount(100) {
		t.Fatalf("transaction changed after rejected update: Amount = %q, want %q", got.Transaction.Amount, cnyAmount(100))
	}
}

func TestUpdateTransactionNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)
	_, categoryID := setupSourceAndCategory(t, session, ledgerID)
	sourceID, _ := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         "999999",
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestDeleteTransactionHappyPath(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(2500), Currency: "CNY", Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &preview)
	if preview.Status != "pending_confirmation" {
		t.Fatalf("Status = %q, want %q", preview.Status, "pending_confirmation")
	}
	if preview.ConfirmationToken == "" {
		t.Fatal("expected a non-empty confirmation_token")
	}

	// Preview must not delete anything.
	var stillThere tools.GetTransactionOutput
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &stillThere)

	var applied tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		LedgerID:          ledgerID,
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &applied)
	if applied.Status != "deleted" {
		t.Fatalf("Status = %q, want %q", applied.Status, "deleted")
	}

	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID})

	var list tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &list)
	for _, txn := range list.Transactions {
		if txn.ID == created.Transaction.ID {
			t.Fatalf("deleted transaction %q still present in search_transactions", created.Transaction.ID)
		}
	}
}

func TestDeleteTransactionNotFound(t *testing.T) {
	session, ledgerID := newTestSession(t)

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: "999999"})
	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: "999999", ConfirmationToken: "irrelevant"})
}

func TestDeleteTransactionExpiredToken(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	expired := craftConfirmationToken(t, testConfirmSecret, "delete_transaction", created.Transaction.ID, "irrelevant-revision", time.Now().Add(-time.Minute).Unix())

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		LedgerID:          ledgerID,
		ID:                created.Transaction.ID,
		ConfirmationToken: expired,
	})

	// The transaction must survive the expired token.
	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestDeleteTransactionDriftedRevisionAfterUpdate(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &preview)

	// The transaction changes after the preview but before apply -- the stale
	// token must be rejected rather than deleting a transaction whose state no
	// longer matches what was previewed.
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         created.Transaction.ID,
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(200),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.UpdateTransactionOutput{})

	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		LedgerID:          ledgerID,
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	})

	callTool(t, session, "get_transaction", tools.GetTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &tools.GetTransactionOutput{})
}

func TestDeleteTransactionTokenReplay(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &created)

	var preview tools.DeleteTransactionOutput
	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{LedgerID: ledgerID, ID: created.Transaction.ID}, &preview)

	callTool(t, session, "delete_transaction", tools.DeleteTransactionInput{
		LedgerID:          ledgerID,
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	}, &tools.DeleteTransactionOutput{})

	// Reusing the same token to confirm the same delete a second time must
	// fail -- the transaction is already gone.
	callToolExpectError(t, session, "delete_transaction", tools.DeleteTransactionInput{
		LedgerID:          ledgerID,
		ID:                created.Transaction.ID,
		ConfirmationToken: preview.ConfirmationToken,
	})
}

func TestSearchTransactionsEmptyResult(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID:  ledgerID,
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
func createExpensesAtOffsets(t *testing.T, session *mcp.ClientSession, ledgerID, sourceID, categoryID string, base int64, offsets []int64) []string {
	t.Helper()
	ids := make([]string, 0, len(offsets))
	for i, off := range offsets {
		var created tools.CreateTransactionOutput
		callTool(t, session, "create_transaction", tools.CreateTransactionInput{
			LedgerID:   ledgerID,
			Type:       "expense",
			SourceID:   sourceID,
			CategoryID: categoryID,
			Amount:     cnyAmount(int64(i + 1)),
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
func createNExpenses(t *testing.T, session *mcp.ClientSession, ledgerID, sourceID, categoryID string, n int, baseTime int64) []string {
	t.Helper()
	offsets := make([]int64, n)
	for i := range offsets {
		offsets[i] = int64(i + 1)
	}
	return createExpensesAtOffsets(t, session, ledgerID, sourceID, categoryID, baseTime, offsets)
}

// TestSearchTransactionsDefaultPageSize verifies the default page size is 50
// (proposal.md / spec.md: "无筛选条件" scenario -- returns the oldest page,
// not everything) by creating more than 50 matching transactions and calling
// search_transactions with limit omitted.
func TestSearchTransactionsDefaultPageSize(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)
	createNExpenses(t, session, ledgerID, sourceID, categoryID, 55, futureTime())

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID}, &out)

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
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)
	createNExpenses(t, session, ledgerID, sourceID, categoryID, 5, futureTime())

	var exactFit tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 5}, &exactFit)
	if len(exactFit.Transactions) != 5 {
		t.Fatalf("expected 5 transactions, got %d", len(exactFit.Transactions))
	}
	if exactFit.NextCursor != "" {
		t.Fatalf("expected no next_cursor when limit exactly fits all results, got %q", exactFit.NextCursor)
	}

	var shortOfFit tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 4}, &shortOfFit)
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
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)
	createNExpenses(t, session, ledgerID, sourceID, categoryID, 12, futureTime())

	var full tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 200}, &full)
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
		callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 5, Cursor: cursor}, &page)
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
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)
	createNExpenses(t, session, ledgerID, sourceID, categoryID, 3, futureTime())

	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Cursor:   "this-is-not-a-valid-cursor",
	})
}

func TestSearchTransactionsCursorRejectedWhenFiltersChange(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)
	createNExpenses(t, session, ledgerID, sourceID, categoryID, 3, futureTime())

	var otherSource tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{
		LedgerID:  ledgerID,
		Operation: "create",
		Name:      "Other Wallet",
	}, &otherSource)

	var page tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 1}, &page)
	if page.NextCursor == "" {
		t.Fatal("expected next_cursor from the first page to exercise the mismatch")
	}

	// Same cursor, but now scoped to a different source_id filter than the
	// one it was issued under -- must be rejected, not silently reused.
	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Limit:    1,
		SourceID: otherSource.Source.ID,
		Cursor:   page.NextCursor,
	})
}

func TestSearchTransactionsLimitOverMaxRejected(t *testing.T) {
	session, ledgerID := newTestSession(t)
	setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 201})
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
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)
	base := futureTime()

	// T1..T4 at base+10, +20, +30, +40.
	ids := createExpensesAtOffsets(t, session, ledgerID, sourceID, categoryID, base, []int64{10, 20, 30, 40})

	var page1 tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 2}, &page1)
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
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(999), Currency: "CNY", Time: base + 5,
	}, &before)
	// - a new transaction inserted after everything so far -- must appear.
	var after tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(998), Currency: "CNY", Time: base + 50,
	}, &after)
	// - T1 (already returned in page1) is updated -- must not reappear or
	//   otherwise disturb pagination. Time is left unchanged so its keyset
	//   position is stable.
	callTool(t, session, "update_transaction", tools.UpdateTransactionInput{
		LedgerID:   ledgerID,
		ID:         ids[0],
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(12345),
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
		callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 2, Cursor: cursor}, &page)
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

func TestCreateTransactionRejectsNonexistentLedger(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   "999999",
		Type:       "expense",
		SourceID:   sourceID,
		CategoryID: categoryID,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestCreateTransactionRejectsSourceFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	_, categoryA := setupSourceAndCategory(t, session, ledgerA)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID
	sourceB := createTestSource(t, session, ledgerB)

	// sourceB belongs to ledgerB, categoryA belongs to ledgerA -- referencing
	// sourceB from a transaction created under ledgerA must be rejected.
	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerA,
		Type:       "expense",
		SourceID:   sourceB,
		CategoryID: categoryA,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestCreateTransactionRejectsCategoryFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceA, _ := setupSourceAndCategory(t, session, ledgerA)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID
	categoryB := createTestCategory(t, session, ledgerB, "Food", "")

	callToolExpectError(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerA,
		Type:       "expense",
		SourceID:   sourceA,
		CategoryID: categoryB,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestGetTransactionCrossLedgerNotFound(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerA)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerA,
		Type:       "expense",
		SourceID:   sourceA,
		CategoryID: categoryA,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &created)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	// The transaction exists, but under ledgerA, not ledgerB -- asking for it
	// under ledgerB must behave exactly like asking for a nonexistent id.
	callToolExpectError(t, session, "get_transaction", tools.GetTransactionInput{
		LedgerID: ledgerB,
		ID:       created.Transaction.ID,
	})
}

func TestUpdateTransactionRejectsSourceFromAnotherLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerA)

	var created tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerA,
		Type:       "expense",
		SourceID:   sourceA,
		CategoryID: categoryA,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &created)

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	sourceB := createTestSource(t, session, ledgerBOut.Ledger.ID)

	callToolExpectError(t, session, "update_transaction", tools.UpdateTransactionInput{
		ID:         created.Transaction.ID,
		LedgerID:   ledgerA,
		Type:       "expense",
		SourceID:   sourceB,
		CategoryID: categoryA,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	})
}

func TestSearchTransactionsIsolatedByLedger(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerA)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerA,
		Type:       "expense",
		SourceID:   sourceA,
		CategoryID: categoryA,
		Amount:     cnyAmount(100),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID
	sourceB, categoryB := setupSourceAndCategory(t, session, ledgerB)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID:   ledgerB,
		Type:       "expense",
		SourceID:   sourceB,
		CategoryID: categoryB,
		Amount:     cnyAmount(200),
		Currency:   "CNY",
		Time:       futureTime(),
	}, &tools.CreateTransactionOutput{})

	var resultsB tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerB}, &resultsB)
	if len(resultsB.Transactions) != 1 {
		t.Fatalf("expected 1 transaction in ledgerB, got %d", len(resultsB.Transactions))
	}
	if resultsB.Transactions[0].Amount != cnyAmount(200) {
		t.Errorf("ledgerB transaction amount = %q, want %q (ledgerA's transaction leaked in)", resultsB.Transactions[0].Amount, cnyAmount(200))
	}
}

func TestSearchTransactionsCursorRejectedAcrossLedgers(t *testing.T) {
	session, ledgerA := newTestSession(t)
	sourceA, categoryA := setupSourceAndCategory(t, session, ledgerA)
	for range 2 {
		callTool(t, session, "create_transaction", tools.CreateTransactionInput{
			LedgerID:   ledgerA,
			Type:       "expense",
			SourceID:   sourceA,
			CategoryID: categoryA,
			Amount:     cnyAmount(100),
			Currency:   "CNY",
			Time:       futureTime(),
		}, &tools.CreateTransactionOutput{})
	}

	var page tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerA, Limit: 1}, &page)
	if page.NextCursor == "" {
		t.Fatal("expected a next_cursor with 2 transactions and limit=1")
	}

	var ledgerBOut tools.ManageLedgerOutput
	callTool(t, session, "manage_ledger", tools.ManageLedgerInput{Operation: "create", Name: "Other Ledger"}, &ledgerBOut)
	ledgerB := ledgerBOut.Ledger.ID

	// A cursor issued under ledgerA must not be replayable under ledgerB --
	// the filter fingerprint includes ledger_id.
	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerB, Cursor: page.NextCursor})
}

// TestSearchTransactionsKeywordMatchesCommentCaseInsensitively covers
// spec.md's "按关键词筛选" scenario: keyword matches comment as a
// case-insensitive substring.
func TestSearchTransactionsKeywordMatchesCommentCaseInsensitively(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var starbucks tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
		Comment: "STARBUCKS #4821",
	}, &starbucks)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime() + 10,
		Comment: "walmart grocery",
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: "starbucks"}, &out)

	if len(out.Transactions) != 1 {
		t.Fatalf("expected 1 transaction matching keyword, got %d", len(out.Transactions))
	}
	if out.Transactions[0].ID != starbucks.Transaction.ID {
		t.Fatalf("matched transaction id = %q, want %q", out.Transactions[0].ID, starbucks.Transaction.ID)
	}
}

// TestSearchTransactionsKeywordMatchesChineseComment exercises keyword
// matching over multi-byte UTF-8 text -- the product's real-world case: per
// the comment discipline, comment holds the raw (typically Chinese) bill
// text verbatim, and precedent retrieval searches it by a Chinese merchant
// substring.
func TestSearchTransactionsKeywordMatchesChineseComment(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var starbucks tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(3200), Currency: "CNY", Time: futureTime(),
		Comment: "星巴克咖啡(南京西路店) 美团支付-支付宝",
	}, &starbucks)
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(8800), Currency: "CNY", Time: futureTime() + 10,
		Comment: "沃尔玛购物",
	}, &tools.CreateTransactionOutput{})

	for _, keyword := range []string{"星巴克", "南京西路", "星巴克咖啡(南京西路店)"} {
		var out tools.SearchTransactionsOutput
		callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: keyword}, &out)
		if len(out.Transactions) != 1 {
			t.Fatalf("keyword %q: expected 1 transaction, got %d", keyword, len(out.Transactions))
		}
		if out.Transactions[0].ID != starbucks.Transaction.ID {
			t.Fatalf("keyword %q: matched transaction id = %q, want %q", keyword, out.Transactions[0].ID, starbucks.Transaction.ID)
		}
	}

	// A keyword that is not a contiguous substring of any comment must not
	// match, even though every individual character appears.
	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: "星巴克 咖啡"}, &out)
	if len(out.Transactions) != 0 {
		t.Fatalf("non-contiguous keyword: expected 0 transactions, got %d", len(out.Transactions))
	}
}

// TestSearchTransactionsKeywordCombinedWithOtherFilters covers spec.md's
// "关键词与其他筛选条件组合" scenario: keyword AND-combines with source_id/
// category_id/start_time/end_time, so only transactions matching every
// provided filter come back.
func TestSearchTransactionsKeywordCombinedWithOtherFilters(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var otherSource tools.ManageSourceOutput
	callTool(t, session, "manage_source", tools.ManageSourceInput{LedgerID: ledgerID, Operation: "create", Name: "Credit Card"}, &otherSource)

	base := futureTime()

	// Matches every filter -- the only one that should be returned.
	var want tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: base,
		Comment: "starbucks coffee",
	}, &want)

	// Matches keyword and time, but the wrong source.
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: otherSource.Source.ID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: base,
		Comment: "starbucks coffee",
	}, &tools.CreateTransactionOutput{})

	// Matches source/category/time, but not the keyword.
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: base,
		Comment: "walmart grocery",
	}, &tools.CreateTransactionOutput{})

	// Matches keyword/source/category, but falls outside the time range.
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: base + 100000,
		Comment: "starbucks coffee",
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID:   ledgerID,
		SourceID:   sourceID,
		CategoryID: categoryID,
		StartTime:  base - 10,
		EndTime:    base + 10,
		Keyword:    "starbucks",
	}, &out)

	if len(out.Transactions) != 1 {
		t.Fatalf("expected 1 transaction matching every filter, got %d", len(out.Transactions))
	}
	if out.Transactions[0].ID != want.Transaction.ID {
		t.Fatalf("matched transaction id = %q, want %q", out.Transactions[0].ID, want.Transaction.ID)
	}
}

// TestSearchTransactionsKeywordWildcardCharactersMatchedLiterally covers
// spec.md's "关键词包含 LIKE 通配符字符" scenario: % and _ in keyword must be
// matched as literal characters, not interpreted as SQL LIKE wildcards.
func TestSearchTransactionsKeywordWildcardCharactersMatchedLiterally(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	var literalMatch tools.CreateTransactionOutput
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
		Comment: "50%_off invoice",
	}, &literalMatch)

	// If % and _ were interpreted as SQL LIKE wildcards instead of literal
	// characters, the pattern %50%_off% would also match this comment
	// (% absorbs "X", _ absorbs the space before "off") -- it must NOT be
	// returned.
	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(200), Currency: "CNY", Time: futureTime() + 10,
		Comment: "50X off invoiceZ",
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: "50%_off"}, &out)

	if len(out.Transactions) != 1 {
		t.Fatalf("expected 1 transaction matching the literal keyword, got %d", len(out.Transactions))
	}
	if out.Transactions[0].ID != literalMatch.Transaction.ID {
		t.Fatalf("matched transaction id = %q, want %q", out.Transactions[0].ID, literalMatch.Transaction.ID)
	}
}

// TestSearchTransactionsBlankKeywordTreatedAsNotProvided covers spec.md's
// "关键词为空白" scenario: an empty or whitespace-only keyword behaves
// exactly like keyword wasn't provided at all.
func TestSearchTransactionsBlankKeywordTreatedAsNotProvided(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
		Comment: "groceries",
	}, &tools.CreateTransactionOutput{})

	for _, keyword := range []string{"", "   ", "\t\n "} {
		var out tools.SearchTransactionsOutput
		callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: keyword}, &out)
		if len(out.Transactions) != 1 {
			t.Fatalf("keyword %q: expected 1 transaction (blank keyword must behave as not provided), got %d", keyword, len(out.Transactions))
		}
	}
}

// TestSearchTransactionsKeywordNoMatchReturnsEmpty covers spec.md's "关键词
// 未命中任何交易" scenario.
func TestSearchTransactionsKeywordNoMatchReturnsEmpty(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	callTool(t, session, "create_transaction", tools.CreateTransactionInput{
		LedgerID: ledgerID,
		Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(100), Currency: "CNY", Time: futureTime(),
		Comment: "groceries",
	}, &tools.CreateTransactionOutput{})

	var out tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: "nonexistent-merchant"}, &out)

	if out.Transactions == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(out.Transactions) != 0 {
		t.Fatalf("expected 0 transactions, got %d", len(out.Transactions))
	}
	if out.NextCursor != "" {
		t.Fatalf("expected no next_cursor when keyword matches nothing, got %q", out.NextCursor)
	}
}

// TestSearchTransactionsKeywordPaginationCoversAllMatches covers spec.md's
// "使用 cursor 翻页" scenario under a keyword filter: paging through with a
// small limit until next_cursor disappears yields exactly the matching set
// (no duplicates, no omissions, same order) as fetching everything in one
// page, even with non-matching transactions interleaved between them.
func TestSearchTransactionsKeywordPaginationCoversAllMatches(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)
	base := futureTime()

	var matchingIDs []string
	for i := range 12 {
		var created tools.CreateTransactionOutput
		callTool(t, session, "create_transaction", tools.CreateTransactionInput{
			LedgerID: ledgerID,
			Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(int64(i + 1)), Currency: "CNY", Time: base + int64(i)*2,
			Comment: "starbucks coffee",
		}, &created)
		matchingIDs = append(matchingIDs, created.Transaction.ID)

		// Interleave a non-matching transaction so keyword filtering must
		// skip it without breaking keyset pagination.
		callTool(t, session, "create_transaction", tools.CreateTransactionInput{
			LedgerID: ledgerID,
			Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(999), Currency: "CNY", Time: base + int64(i)*2 + 1,
			Comment: "walmart grocery",
		}, &tools.CreateTransactionOutput{})
	}

	var full tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: "starbucks", Limit: 200}, &full)
	if len(full.Transactions) != 12 {
		t.Fatalf("expected 12 matching transactions unpaginated, got %d", len(full.Transactions))
	}

	var paged []tools.TransactionInfo
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("pagination did not terminate")
		}
		var page tools.SearchTransactionsOutput
		callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: "starbucks", Limit: 5, Cursor: cursor}, &page)
		paged = append(paged, page.Transactions...)
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	if len(paged) != len(matchingIDs) {
		t.Fatalf("paginated total = %d, want %d", len(paged), len(matchingIDs))
	}
	seen := map[string]bool{}
	for i, txn := range paged {
		if seen[txn.ID] {
			t.Fatalf("transaction %q returned more than once across pages", txn.ID)
		}
		seen[txn.ID] = true
		if txn.ID != matchingIDs[i] {
			t.Fatalf("paged[%d].ID = %q, want %q", i, txn.ID, matchingIDs[i])
		}
	}
}

// TestSearchTransactionsCursorRejectedWhenKeywordChanges covers spec.md's
// "cursor 无效或已不匹配当前筛选条件" scenario for keyword: a cursor issued
// under one keyword (including "no keyword") must be rejected when replayed
// under a different one.
func TestSearchTransactionsCursorRejectedWhenKeywordChanges(t *testing.T) {
	session, ledgerID := newTestSession(t)
	sourceID, categoryID := setupSourceAndCategory(t, session, ledgerID)

	for i := range 3 {
		callTool(t, session, "create_transaction", tools.CreateTransactionInput{
			LedgerID: ledgerID,
			Type:     "expense", SourceID: sourceID, CategoryID: categoryID, Amount: cnyAmount(int64(i + 1)), Currency: "CNY", Time: futureTime() + int64(i),
			Comment: "starbucks coffee",
		}, &tools.CreateTransactionOutput{})
	}

	var page tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Keyword: "starbucks", Limit: 1}, &page)
	if page.NextCursor == "" {
		t.Fatal("expected a next_cursor to exercise the keyword mismatch")
	}

	// A different keyword than the one the cursor was issued under.
	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Keyword:  "walmart",
		Limit:    1,
		Cursor:   page.NextCursor,
	})

	// No keyword at all.
	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Limit:    1,
		Cursor:   page.NextCursor,
	})

	// The reverse: a cursor issued without a keyword must be rejected when
	// replayed with one.
	var noKeywordPage tools.SearchTransactionsOutput
	callTool(t, session, "search_transactions", tools.SearchTransactionsInput{LedgerID: ledgerID, Limit: 1}, &noKeywordPage)
	if noKeywordPage.NextCursor == "" {
		t.Fatal("expected a next_cursor from the no-keyword page")
	}
	callToolExpectError(t, session, "search_transactions", tools.SearchTransactionsInput{
		LedgerID: ledgerID,
		Keyword:  "starbucks",
		Limit:    1,
		Cursor:   noKeywordPage.NextCursor,
	})
}
